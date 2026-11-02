package repository

import (
	"cmp"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// An Offer is one action of a worktree's menu: the follow-up that carries it out, or why the worktree does not offer it.
type Offer struct {
	Action   worktree.Action     `json:"action"`
	FollowUp *domain.FollowUp    `json:"follow_up"` // null when the worktree does not offer it
	Refused  *worktree.Unoffered `json:"refused"`   // null when it offers it
}

// OffersParams are the worktree whose menu a read reports: the read's name, which prefixes each follow-up's code, the worktree's branch ("" on none) and path, its pull request's number and URL (0 and "" when none is known), whether it is the main worktree, on the base branch, or its directory gone.
type OffersParams struct {
	Command  string
	Branch   string
	Path     string
	PRNumber int
	PRURL    string
	Main     bool
	Base     bool
	Missing  bool
	OnOrigin bool
	Record   worktree.RecordState
}

// ParamNumber is the param of the follow-up that opens a pull request: its number, beside its URL.
const ParamNumber = "number"

// byAction is what each action runs, the wtm command as its command line spells it after the program's name, none to open a pull request, and what it reads as in a picker and as the note of its follow-up.
var byAction = map[worktree.Action]struct{ command, label string }{
	worktree.ActionGo:          {domain.CommandGo, "Go to the worktree"},
	worktree.ActionOpenPR:      {"", "Open the pull request"},
	worktree.ActionExec:        {domain.CommandExec, "Run a command"},
	worktree.ActionRunUp:       {domain.CommandRunUp, "Start the profile"},
	worktree.ActionRunDown:     {"run down", "Stop the profile"},
	worktree.ActionLogs:        {"run logs", "View the logs"},
	worktree.ActionFastForward: {domain.CommandFastForward, "Fast-forward from origin"},
	worktree.ActionSync:        {domain.CommandSync, "Sync onto its parent"},
	worktree.ActionReparent:    {domain.CommandReparent, "Change the parent"},
	worktree.ActionClean:       {domain.CommandClean, "Clean (delete the worktree)"},
}

// Offers is every action of the worktree of p, in worktree.Actions' order, each with its follow-up or why it is not offered.
func Offers(p OffersParams) []Offer {
	facts := worktree.OfferParams{Main: p.Main, Base: p.Base, Missing: p.Missing, Detached: p.Branch == "", PullRequest: p.PRURL != "", OnOrigin: p.OnOrigin, Record: p.Record}
	offers := make([]Offer, 0, len(byAction))
	for _, action := range worktree.Actions() {
		if refused := worktree.Unoffer(facts, action); refused != "" {
			offers = append(offers, Offer{Action: action, Refused: &refused})
			continue
		}
		followUp := p.followUpOf(action)
		offers = append(offers, Offer{Action: action, FollowUp: &followUp})
	}

	return offers
}

// Offered are the follow-ups of the actions offers offers.
func Offered(offers []Offer) []domain.FollowUp {
	var followUps []domain.FollowUp
	for _, offer := range offers {
		if offer.FollowUp != nil {
			followUps = append(followUps, *offer.FollowUp)
		}
	}

	return followUps
}

func (p OffersParams) followUpOf(action worktree.Action) domain.FollowUp {
	followUp := domain.FollowUp{Code: p.Command + "." + string(action), Message: byAction[action].label}
	if action == worktree.ActionOpenPR {
		followUp.URL, followUp.Params = p.PRURL, domain.Params{ParamNumber: strconv.Itoa(p.PRNumber)}
		return followUp
	}

	followUp.Command, followUp.Answers = byAction[action].command, []domain.Answered{{Path: domain.PathWorktree, Values: []string{cmp.Or(p.Branch, p.Path)}}}
	return followUp
}
