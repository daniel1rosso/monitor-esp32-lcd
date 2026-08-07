package buildinfo

// Info identifies a reproducible backend build.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at"`
}

func New(version, commit, builtAt string) Info {
	return Info{Version: version, Commit: commit, BuiltAt: builtAt}
}
