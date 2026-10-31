package sync

// PlanEmpty reports whether applying p changes nothing, as Plan sets Recapped.Empty.
var PlanEmpty = Plan.empty

// FollowUpsOf is the steps a run of p offers after items, as Apply sets Result.FollowUps.
var FollowUpsOf = Plan.next
