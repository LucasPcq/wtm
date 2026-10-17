package branch

type Clash struct {
	Branch string
	Name   string
}

type ClashParams struct {
	Branch string
	Live   []string
}

func ClashWith(params ClashParams) (Clash, bool) {
	slug := Slug(params.Branch)
	host := HostLabel(slug)
	for _, live := range params.Live {
		if live == "" || live == params.Branch {
			continue
		}
		other := Slug(live)
		if other == slug {
			return Clash{Branch: live, Name: slug}, true
		}
		if HostLabel(other) == host {
			return Clash{Branch: live, Name: host}, true
		}
	}
	return Clash{}, false
}
