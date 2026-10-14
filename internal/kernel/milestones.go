package kernel

type Committed[D any] struct {
	Subject string
	Detail  D
}

type PhaseDone[D any] struct {
	Subject string
	Phase   string
	Detail  D
}

type Milestones[D any] struct {
	Committed func(Committed[D])
	PhaseDone func(PhaseDone[D])
	Finished  func(Item[D])
}

func (m Milestones[D]) committed(milestone Committed[D]) {
	if m.Committed != nil {
		m.Committed(milestone)
	}
}

func (m Milestones[D]) phaseDone(milestone PhaseDone[D]) {
	if m.PhaseDone != nil {
		m.PhaseDone(milestone)
	}
}

func (m Milestones[D]) finished(item Item[D]) {
	if m.Finished != nil {
		m.Finished(item)
	}
}
