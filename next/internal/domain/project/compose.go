package project

import "errors"

// ErrComposeChanged refuses a rewrite of a docker compose file that changed on disk since it was read.
var ErrComposeChanged = errors.New("the docker compose file changed on disk since it was read")

// Compose is a repository's docker compose file as read: its path relative to the repository ("" when it has none), the digest of its bytes, its services in the file's order, the absolute names it gives a container, a volume or a network, and the variables it reads anywhere. Err says why a file there does not parse.
type Compose struct {
	File     string
	Digest   string
	Services []ComposeService
	Names    []ComposeName
	Reads    []string
	Err      string
}

// ComposeService is a service of a docker compose file: its name, its image ("" when it is built here, Build), and the ports it publishes, in order.
type ComposeService struct {
	Name  string
	Image string
	Build bool
	Ports []ComposePort
}

// ComposePort is one port a service publishes, as the file spells it: the listen address before it ("" for every address), its host side ("5432", "${PG_PORT:-5432}"), its container side, and where its host side is written, to be rewritten; Unreadable says why it cannot be (an anchor, an alias, a host side wtm cannot find in its line).
type ComposePort struct {
	IP         string
	Host       string
	Container  string
	At         ComposeSpan
	Unreadable string
}

// ComposeNameKind is what an absolute name of a docker compose file names.
type ComposeNameKind string

// The kinds of absolute name.
const (
	ComposeContainer ComposeNameKind = "container"
	ComposeVolume    ComposeNameKind = "volume"
	ComposeNetwork   ComposeNameKind = "network"
)

// ComposeName is a name a docker compose file pins outside its project: the container_name of the service Owner, or the name of the volume or network Owner, and where it is written; Unreadable says why it cannot be rewritten.
type ComposeName struct {
	Kind       ComposeNameKind
	Owner      string
	Value      string
	At         ComposeSpan
	Unreadable string
}

// ComposeSpan is a value written in a docker compose file: its line and column (from 1), and its text exactly as written there, quotes included.
type ComposeSpan struct {
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Text   string `json:"text"`
}

// ComposeEdit replaces the value written at At with New.
type ComposeEdit struct {
	At  ComposeSpan
	New string
}

// ComposeRewrite is a docker compose file rewritten in place: its edits, and the digest of the file they apply to, as Compose read it.
type ComposeRewrite struct {
	File     string
	Edits    []ComposeEdit
	Replaces string
}

// ComposeFileNames are the names of a docker compose file, in the order docker compose looks for one.
func ComposeFileNames() []string {
	return []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}
}
