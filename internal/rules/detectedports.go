package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ResolveDetectedPortsParams struct {
	Answers        domain.InitProjectAnswers
	PackageManager domain.PackageManager
	Existing       domain.RunConfig
	Plan           ComposePortPlan
	// Unverifiable maps a compose file to the reason its recorded positions can
	// no longer be trusted. Such a file contributes nothing: a stale scan would
	// declare ports the file may no longer expose.
	Unverifiable map[string]string
	// EnvScansByDir holds the ports each directory's env files declare, keyed by
	// the directory a job's cwd names.
	EnvScansByDir map[string]domain.EnvPortScan
	// Deselected are the jobs the reader unchecked. They are dropped right after
	// the merge, because every later step reads the config this returns.
	Deselected []string
}

// DetectedPortsOutcome is the whole decision of a `run init`. Nothing here
// touches disk, so the order of the steps below is testable on its own.
type DetectedPortsOutcome struct {
	Config  domain.RunConfig
	Merge   MergeResult
	Patches map[string][]domain.ComposePortBinding

	Written    map[string]map[string]int
	Withheld   []domain.ComposePortBinding
	Dropped    []DroppedPort
	Unreadable []domain.ComposeScan
	// Changed and Orphaned name the files that contributed nothing, and why.
	Changed  map[string]string
	Orphaned []string
	// Removed are the jobs the unchecking dropped, so a surface can name them
	// before the config is written.
	Removed []string

	// EnvWritten is what the .env detection gave each job, and EnvSources the
	// file each of those ports was read from. Kept apart from Written: the two
	// sources warrant different advice, a compose mapping being honoured by
	// Docker while a dev server has to read the variable itself.
	EnvWritten map[string]map[string]int
	EnvSources map[string]map[string]string
	// EnvUnreadable are the directories whose env files could not be read.
	EnvUnreadable []domain.EnvPortScan
}

// ResolveDetectedPorts runs the whole sequence in one place: build the jobs the
// selection still needs, merge, backfill the detected ports, withdraw the ones
// that cannot coexist, and keep only the rewrites whose declaration survived.
//
// The order matters and is the point of gathering it here. A mapping is only
// ever rewritten once its declaration is known to be kept — otherwise wtm would
// edit a project file for a port it then refuses to declare.
func ResolveDetectedPorts(params ResolveDetectedPortsParams) DetectedPortsOutcome {
	outcome := DetectedPortsOutcome{
		Withheld:   params.Plan.Withheld,
		Unreadable: params.Plan.Unreadable,
		Changed:    params.Unverifiable,
	}

	ports, patches := withoutFiles(params.Plan, sortedKeys(params.Unverifiable))

	forJobs := params.Answers
	forJobs.DockerComposeFiles = ComposeFilesNeedingAJob(ComposeFilesNeedingAJobParams{
		Config: params.Existing,
		Files:  params.Answers.DockerComposeFiles,
	})
	built := BuildInitRunConfig(forJobs, params.PackageManager)
	merged, mergeResult := MergeRunConfigs(params.Existing, built)
	outcome.Merge = mergeResult

	for _, name := range params.Deselected {
		next, effect := RemoveJob(merged, name)
		if !effect.Removed {
			continue
		}
		merged = next
		outcome.Removed = append(outcome.Removed, name)
	}

	// A file whose job was skipped — its name already taken by another file —
	// has nowhere to put its ports. Declaring them elsewhere would be a guess.
	outcome.Orphaned = orphanedComposeFiles(merged, ports)
	ports, patches = withoutFiles(ComposePortPlan{PortsByFile: ports, Patches: patches}, outcome.Orphaned)

	// A job stacking several compose files (-f base.yml -f dev.yml) receives all
	// of their ports at once, so the same variable can arrive twice with two
	// bases — the per-file check inside the plan cannot see it.
	conflicts := sharedVarsAcrossFiles(merged, ports)
	ports, patches = withoutVars(ports, patches, conflicts)
	outcome.Withheld = append(outcome.Withheld, withheldForConflicts(withheldForConflictsParams{
		Config:    merged,
		Declared:  params.Plan.Declared,
		Conflicts: conflicts,
	})...)

	// Applied to the merged config, not only to the jobs just built: a re-init
	// never rebuilds a compose file that already has a job, so the scope answers
	// would otherwise reach nothing at all.
	merged = ApplySharedServices(ApplySharedServicesParams{
		Config:     merged,
		Shared:     params.Answers.SharedServices,
		Asked:      params.Answers.ScopesAsked,
		Scans:      params.Answers.Scans,
		Bindings:   params.Plan.Declared,
		ComposeCmd: params.Answers.DockerComposeCmd,
	})

	// After the namespaces are settled and before the ports are backfilled: a
	// link reads the slice the step above just named, and may take an
	// [[env_port]] off a key it now writes in full.
	merged = ApplyEnvValues(ApplyEnvValuesParams{
		Config:  merged,
		Values:  params.Answers.EnvValues,
		Asked:   params.Answers.EnvValuesAsked,
		Offered: params.Answers.EnvValuesOffered,
	})

	backfilled := BackfillDockerPorts(BackfillDockerPortsParams{
		Config:      merged,
		PortsByFile: ports,
		Declared:    params.Plan.Declared,
		Shared:      params.Answers.SharedServices,
	})
	fromEnv := BackfillScriptPorts(BackfillScriptPortsParams{
		Config:         backfilled.Config,
		PackageManager: params.PackageManager,
		Scripts:        params.Answers.SelectedPackageScripts,
		ScansByDir:     params.EnvScansByDir,
	})

	// Both detections are pruned together: a compose port and a .env port can
	// meet just as two compose ports can, and only one pass sees both.
	pruned := PruneCollidingPorts(PruneCollidingPortsParams{
		Config:   fromEnv.Config,
		Detected: mergeDetected(backfilled.Added, fromEnv.Added),
	})

	outcome.Config = pruned.Config
	outcome.Dropped = pruned.Dropped
	outcome.Written = RemoveDroppedPorts(RemoveDroppedPortsParams{Added: backfilled.Added, Dropped: pruned.Dropped})
	outcome.EnvWritten = RemoveDroppedPorts(RemoveDroppedPortsParams{Added: fromEnv.Added, Dropped: pruned.Dropped})
	outcome.EnvSources = fromEnv.Sources
	outcome.EnvUnreadable = unreadableEnvScans(params.EnvScansByDir)
	outcome.Patches = survivingPatches(pruned.Config, patches)
	return outcome
}

// mergeDetected unions the two backfills for the prune. A job can appear in
// both — a compose job never gains a .env port, but the map is keyed by job and
// nothing structural forbids it.
func mergeDetected(fromCompose, fromEnv map[string]map[string]int) map[string]map[string]int {
	merged := make(map[string]map[string]int, len(fromCompose)+len(fromEnv))
	for _, source := range []map[string]map[string]int{fromCompose, fromEnv} {
		for job, ports := range source {
			if merged[job] == nil {
				merged[job] = map[string]int{}
			}
			for name, base := range ports {
				merged[job][name] = base
			}
		}
	}
	return merged
}

func unreadableEnvScans(scans map[string]domain.EnvPortScan) []domain.EnvPortScan {
	var unreadable []domain.EnvPortScan
	for _, dir := range sortedKeys(scans) {
		if scans[dir].Err != "" {
			unreadable = append(unreadable, scans[dir])
		}
	}
	return unreadable
}

// survivingPatches keeps a rewrite only when the config about to be written
// declares its variable at exactly the base the mapping had. A different base —
// one the user wrote by hand, or one a second file won — would make the rewrite
// move a binding that already worked, which is the whole thing to avoid.
func survivingPatches(cfg domain.RunConfig, patches map[string][]domain.ComposePortBinding) map[string][]domain.ComposePortBinding {
	kept := map[string][]domain.ComposePortBinding{}
	for _, file := range SortedComposeFiles(patches) {
		jobs := ComposeJobsRunningFile(cfg, file)
		for _, binding := range patches[file] {
			if declaredAt(jobs, binding) {
				kept[file] = append(kept[file], binding)
			}
		}
	}
	return kept
}

// declaredAt says one of the jobs running the file declares the variable at the
// base the mapping has. Any of them answers: a lifted service took its own
// ports out of the file's job, and the rewrite is about the mapping, not about
// which job ended up holding it.
func declaredAt(jobs []domain.JobConfig, binding domain.ComposePortBinding) bool {
	for _, job := range jobs {
		if base, declared := job.Ports[binding.Var]; declared && base == binding.Base {
			return true
		}
	}
	return false
}

// sharedVarsAcrossFiles maps a job to the variables its files disagree on.
func sharedVarsAcrossFiles(cfg domain.RunConfig, ports map[string]map[string]int) map[string]map[string]bool {
	seen := map[string]map[string]int{}
	conflicts := map[string]map[string]bool{}

	for _, file := range SortedComposeFiles(ports) {
		job := ComposeJobName(ComposeJobNameParams{Config: cfg, File: file})
		if job == "" {
			continue
		}
		if seen[job] == nil {
			seen[job] = map[string]int{}
		}
		for _, name := range sortedPortNames(ports[file]) {
			base := ports[file][name]
			if previous, found := seen[job][name]; found && previous != base {
				if conflicts[job] == nil {
					conflicts[job] = map[string]bool{}
				}
				conflicts[job][name] = true
				continue
			}
			seen[job][name] = base
		}
	}
	return conflicts
}

func withoutVars(
	ports map[string]map[string]int,
	patches map[string][]domain.ComposePortBinding,
	conflicts map[string]map[string]bool,
) (map[string]map[string]int, map[string][]domain.ComposePortBinding) {
	if len(conflicts) == 0 {
		return ports, patches
	}

	inConflict := map[string]bool{}
	for _, vars := range conflicts {
		for name := range vars {
			inConflict[name] = true
		}
	}

	keptPorts := map[string]map[string]int{}
	for file, byName := range ports {
		kept := map[string]int{}
		for name, base := range byName {
			if !inConflict[name] {
				kept[name] = base
			}
		}
		if len(kept) > 0 {
			keptPorts[file] = kept
		}
	}

	keptPatches := map[string][]domain.ComposePortBinding{}
	for file, bindings := range patches {
		for _, b := range bindings {
			if !inConflict[b.Var] {
				keptPatches[file] = append(keptPatches[file], b)
			}
		}
	}
	return keptPorts, keptPatches
}

type withheldForConflictsParams struct {
	Config    domain.RunConfig
	Declared  map[string][]domain.ComposePortBinding
	Conflicts map[string]map[string]bool
}

func withheldForConflicts(params withheldForConflictsParams) []domain.ComposePortBinding {
	var withheld []domain.ComposePortBinding
	for _, file := range SortedComposeFiles(params.Declared) {
		job := ComposeJobName(ComposeJobNameParams{Config: params.Config, File: file})
		for _, binding := range params.Declared[file] {
			if !params.Conflicts[job][binding.Var] {
				continue
			}
			binding.Reason = fmt.Sprintf(domain.ComposePortReasonSharedJob, binding.Var, job)
			withheld = append(withheld, binding)
		}
	}
	return withheld
}

func orphanedComposeFiles(cfg domain.RunConfig, ports map[string]map[string]int) []string {
	var orphaned []string
	for _, file := range SortedComposeFiles(ports) {
		if len(ports[file]) > 0 && len(ComposeJobsRunningFile(cfg, file)) == 0 {
			orphaned = append(orphaned, file)
		}
	}
	return orphaned
}

func withoutFiles(plan ComposePortPlan, files []string) (map[string]map[string]int, map[string][]domain.ComposePortBinding) {
	if len(files) == 0 {
		return plan.PortsByFile, plan.Patches
	}

	excluded := make(map[string]bool, len(files))
	for _, f := range files {
		excluded[f] = true
	}

	ports := map[string]map[string]int{}
	for file, p := range plan.PortsByFile {
		if !excluded[file] {
			ports[file] = p
		}
	}
	patches := map[string][]domain.ComposePortBinding{}
	for file, b := range plan.Patches {
		if !excluded[file] {
			patches[file] = b
		}
	}
	return ports, patches
}

func jobNamed(cfg domain.RunConfig, name string) domain.JobConfig {
	for _, job := range cfg.Jobs {
		if job.Name == name {
			return job
		}
	}
	return domain.JobConfig{}
}
