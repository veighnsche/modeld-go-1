package testctl

// Indirection layer to allow stubbing in tests

var (
	fnInstallJS         = installJS
	fnInstallGo         = installGo
	fnInstallPy         = installPy
	fnInstallHostDocker = installHostDocker
	fnInstallHostAct    = installHostAct

	fnRunGoTests = runGoTests
	fnRunPyTests = runPyTests

	fnTestWebMock     = testWebMock
	fnTestWebLiveHost = testWebLiveHost

	fnHasHostModels = hasHostModels

	// CI helpers
	fnRunCIAll      = runCIAll
	fnRunCIWorkflow = runCIWorkflow
)
