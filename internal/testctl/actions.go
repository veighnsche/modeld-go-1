package testctl

// Indirection layer to allow stubbing in tests

var (
	fnInstallNodeJS     = installNodeJS
	fnInstallGo         = installGo
	fnInstallPy         = installPy
	fnInstallHostDocker = installHostDocker
	fnInstallHostAct    = installHostAct
	fnInstallLlamaCUDA  = installLlamaCUDA

	fnRunGoTests = runGoTests
	fnRunPyTests = runPyTests
	fnRunPyTestHaiku = runPyTestHaiku

	fnTestWebMock     = testWebMock
	fnTestWebLiveHost = testWebLiveHost
	fnTestWebHaikuHost = testWebHaikuHost

	fnHasHostModels = hasHostModels

	// CI helpers
	fnRunCIAll      = runCIAll
	fnRunCIWorkflow = runCIWorkflow
)
