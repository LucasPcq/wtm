package process

func newManager() *Manager { return newManagerWithRoutes(nil) }

func newManagerWithRoutes(routes RouteSink) *Manager {
	return NewManagerWith(ManagerParams{Routes: routes})
}
