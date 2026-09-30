package rules

import (
	"sort"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ForestNode is an enriched worktree fed into BuildForest: the parent link
// (SourceBranch), the creation time (for sibling ordering) and the per-node
// status flags already computed by the service layer.
type ForestNode struct {
	Branch       string
	Path         string
	SourceBranch string
	IsMain       bool
	CreatedAt    time.Time
	Status       domain.TreeNodeStatus
}

// BuildForestParams holds the inputs for assembling the rendered forest.
type BuildForestParams struct {
	Nodes []ForestNode
}

// BuildForest assembles a forest of worktrees from their parent links. Unlike
// BuildSyncPlan it never fails: a parent with no managed worktree becomes a
// grey "virtual root", and a cycle in the parent chain is broken (the looping
// node is promoted to a root and flagged InCycle) so the tree stays finite.
//
// Roots are ordered main-first, then real orphan roots by creation date, then
// virtual roots alphabetically. Siblings are ordered by creation date.
func BuildForest(params BuildForestParams) domain.Forest {
	managed := make(map[string]ForestNode, len(params.Nodes))
	for _, n := range params.Nodes {
		managed[n.Branch] = n
	}

	inCycle := detectForestCycles(managed)

	childrenByParent := make(map[string][]ForestNode)
	virtualParents := make(map[string]struct{})
	var realRoots []ForestNode

	for _, n := range params.Nodes {
		if n.IsMain || n.SourceBranch == "" || inCycle[n.Branch] {
			realRoots = append(realRoots, n)
			continue
		}
		if _, ok := managed[n.SourceBranch]; ok {
			childrenByParent[n.SourceBranch] = append(childrenByParent[n.SourceBranch], n)
			continue
		}
		// Parent branch exists in metadata but has no managed worktree: group its
		// children under a virtual root standing in for the unmanaged branch.
		virtualParents[n.SourceBranch] = struct{}{}
		childrenByParent[n.SourceBranch] = append(childrenByParent[n.SourceBranch], n)
	}

	b := forestBuilder{
		childrenByParent: childrenByParent,
		inCycle:          inCycle,
		visited:          make(map[string]struct{}, len(params.Nodes)),
	}

	sortRealRoots(realRoots)
	roots := make([]domain.TreeNode, 0, len(realRoots)+len(virtualParents))
	for _, r := range realRoots {
		roots = append(roots, b.build(r))
	}

	virtualNames := make([]string, 0, len(virtualParents))
	for name := range virtualParents {
		virtualNames = append(virtualNames, name)
	}
	sort.Strings(virtualNames)
	for _, name := range virtualNames {
		roots = append(roots, b.buildVirtual(name))
	}

	return domain.Forest{Roots: roots}
}

type forestBuilder struct {
	childrenByParent map[string][]ForestNode
	inCycle          map[string]bool
	visited          map[string]struct{}
}

func (b *forestBuilder) build(n ForestNode) domain.TreeNode {
	b.visited[n.Branch] = struct{}{}

	status := n.Status
	if b.inCycle[n.Branch] {
		status.InCycle = true
	}

	node := domain.TreeNode{
		Branch:   n.Branch,
		Path:     n.Path,
		IsMain:   n.IsMain,
		Status:   status,
		Children: b.childrenOf(n.Branch),
	}
	return node
}

func (b *forestBuilder) buildVirtual(name string) domain.TreeNode {
	b.visited[name] = struct{}{}
	return domain.TreeNode{
		Branch:    name,
		IsVirtual: true,
		Children:  b.childrenOf(name),
	}
}

func (b *forestBuilder) childrenOf(parent string) []domain.TreeNode {
	children := b.childrenByParent[parent]
	sortSiblings(children)

	out := make([]domain.TreeNode, 0, len(children))
	for _, child := range children {
		if _, seen := b.visited[child.Branch]; seen {
			continue
		}
		out = append(out, b.build(child))
	}
	return out
}

// detectForestCycles reports which branches sit on a parent cycle, i.e. walking
// the SourceBranch chain returns to the branch itself.
func detectForestCycles(managed map[string]ForestNode) map[string]bool {
	inCycle := make(map[string]bool)
	for branch := range managed {
		seen := make(map[string]struct{})
		current := branch
		for {
			node, ok := managed[current]
			if !ok {
				break
			}
			if _, looped := seen[current]; looped {
				break
			}
			seen[current] = struct{}{}
			current = node.SourceBranch
			if current == branch {
				inCycle[branch] = true
				break
			}
		}
	}
	return inCycle
}

// sortRealRoots orders roots main-first then by creation date (oldest first).
func sortRealRoots(roots []ForestNode) {
	sort.SliceStable(roots, func(i, j int) bool {
		if roots[i].IsMain != roots[j].IsMain {
			return roots[i].IsMain
		}
		return roots[i].CreatedAt.Before(roots[j].CreatedAt)
	})
}

// sortSiblings orders children by creation date (oldest first).
func sortSiblings(nodes []ForestNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].CreatedAt.Before(nodes[j].CreatedAt)
	})
}

// FlattenForest walks the forest depth-first into one row per node, each carrying
// the connector gutter its position earned it. Both `wtm tree` and the dashboard
// draw from it, so the shape of the tree is decided here rather than once per
// surface.
func FlattenForest(forest domain.Forest) []domain.TreeRow {
	rows := make([]domain.TreeRow, 0, len(forest.Roots))
	for _, root := range forest.Roots {
		rows = appendTreeRows(appendTreeRowsParams{Node: root, Rows: rows})
	}
	return rows
}

type appendTreeRowsParams struct {
	Node domain.TreeNode
	Rows []domain.TreeRow
	// Gutter is what the ancestors contribute, Connector this node's own glyph
	// (empty for a root).
	Gutter    string
	Connector string
	Depth     int
}

func appendTreeRows(params appendTreeRowsParams) []domain.TreeRow {
	rows := append(params.Rows, domain.TreeRow{
		Node:   params.Node,
		Prefix: params.Gutter + params.Connector,
		Depth:  params.Depth,
	})

	childGutter := params.Gutter
	switch params.Connector {
	case domain.TreeConnectorBranch:
		childGutter += domain.TreeGutterPipe
	case domain.TreeConnectorLast:
		childGutter += domain.TreeGutterBlank
	}

	for index, child := range params.Node.Children {
		connector := domain.TreeConnectorBranch
		if index == len(params.Node.Children)-1 {
			connector = domain.TreeConnectorLast
		}
		rows = appendTreeRows(appendTreeRowsParams{
			Node:      child,
			Rows:      rows,
			Gutter:    childGutter,
			Connector: connector,
			Depth:     params.Depth + 1,
		})
	}
	return rows
}

// TreeBadges enumerates a node's status annotations in their canonical display
// order. It is the single source of truth for which badges appear and in what
// order; the ASCII tree, the Mermaid export and the dashboard differ only in how
// they style each one, so they cannot drift apart.
func TreeBadges(node domain.TreeNode) []domain.TreeBadge {
	badges := make([]domain.TreeBadge, 0, 8)
	if node.IsVirtual {
		badges = append(badges, domain.TreeBadgeVirtual)
	}
	if node.Status.RunningJobs > 0 {
		badges = append(badges, domain.TreeBadgeRunning)
	}
	if node.Status.PR != nil {
		badges = append(badges, domain.TreeBadgePR)
	}
	if node.Status.CommitsAhead > 0 {
		badges = append(badges, domain.TreeBadgeAhead)
	}
	if HasOriginDivergence(node.Status.OriginState) {
		badges = append(badges, domain.TreeBadgeOrigin)
	}
	if node.Status.RebaseInProgress {
		badges = append(badges, domain.TreeBadgeRebasing)
	} else if node.Status.IsDirty {
		badges = append(badges, domain.TreeBadgeDirty)
	}
	if node.Status.NeedsSync {
		badges = append(badges, domain.TreeBadgeNeedsSync)
	}
	if node.Status.InCycle {
		badges = append(badges, domain.TreeBadgeCycle)
	}
	return badges
}

// HasOriginDivergence reports whether an origin state warrants a badge;
// up-to-date and unknown show none.
func HasOriginDivergence(state domain.DivergenceState) bool {
	switch state {
	case domain.DivergenceBehind, domain.DivergenceAhead, domain.DivergenceDiverged:
		return true
	default:
		return false
	}
}

// TreeSpacerPrefix is the gutter of the blank line drawn between two tree rows:
// the pipe runs down to and past the last sibling, so the structure stays
// connected while the rows breathe. A row that starts a new root gets a truly
// blank line, which is what separates one tree from the next.
func TreeSpacerPrefix(nextPrefix string) string {
	for _, connector := range []string{domain.TreeConnectorBranch, domain.TreeConnectorLast} {
		if strings.HasSuffix(nextPrefix, connector) {
			return nextPrefix[:len(nextPrefix)-len(connector)] + domain.TreeGutterPipe
		}
	}
	return ""
}

// ForestWithRunningJobs stamps each node with what the run daemon holds for its
// worktree. The count travels on the node rather than beside it so every
// renderer of a forest draws the badge from the same place.
func ForestWithRunningJobs(forest domain.Forest, running map[string]int) domain.Forest {
	if len(running) == 0 {
		return forest
	}
	roots := make([]domain.TreeNode, 0, len(forest.Roots))
	for _, root := range forest.Roots {
		roots = append(roots, nodeWithRunningJobs(root, running))
	}
	return domain.Forest{Roots: roots}
}

func nodeWithRunningJobs(node domain.TreeNode, running map[string]int) domain.TreeNode {
	node.Status.RunningJobs = running[node.Path]
	children := make([]domain.TreeNode, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, nodeWithRunningJobs(child, running))
	}
	node.Children = children
	return node
}
