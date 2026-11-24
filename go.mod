module bikegeo

go 1.27.2

require github.com/PuerkitoBio/goquery v1.14.0

require (
	github.com/andybalholm/cascadia v1.3.5 // indirect
	github.com/nishanths/exhaustive v0.17.1 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20261009151841-97e8ea77aa2a // indirect
	golang.org/x/tools v0.52.0 // indirect
)

tool (
	github.com/nishanths/exhaustive/cmd/exhaustive
	golang.org/x/tools/cmd/deadcode
)
