package flags

// version will be injected using -ldflags in the build process
var version = "dev"

// commit will be injected using -ldflags in the build process
var commit = ""

func GetVersion() string {
	return version
}

func GetCommit() string {
	return commit
}

func GetShortCommit() string {
	if len(commit) >= 7 {
		return commit[0:7]
	}
	return commit
}
