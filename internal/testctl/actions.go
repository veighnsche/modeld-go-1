package testctl

// Indirection layer to allow stubbing in tests

var (
	fnInstallNodeJS     = installNodeJS
	fnInstallGo         = installGo
	fnInstallPy         = installPy
	fnInstallHostDocker = installHostDocker
	fnInstallHostAct    = installHostAct
	fnInstallLlamaCUDA  = installLlamaCUDA

	fnRunGoTests     = runGoTests
	fnRunPyTests     = runPyTests
	fnRunPyTestHaiku = runPyTestHaiku

	fnTestWebLiveHost  = testWebLiveHost
	fnTestWebHaikuHost = testWebHaikuHost

	fnHasHostModels = hasHostModels

	// CI helpers
	fnRunCIAll      = runCIAll
	fnRunCIWorkflow = runCIWorkflow

	// Verify helpers
	fnVerifyHostDocker = verifyHostDocker
	fnVerifyHostAct    = verifyHostAct

	// CI installer helpers (install + verify)
	fnRunCIInstallersAll    = runCIInstallersAll
	fnRunCIInstallersAct    = runCIInstallersAct
	fnRunCIInstallersDocker = runCIInstallersDocker
)
