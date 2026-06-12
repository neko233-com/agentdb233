package version

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String(name string) string {
	return name + " " + Version + " (" + Commit + ", " + Date + ")"
}
