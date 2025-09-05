package testctl

// Indirection layer to allow stubbing in tests

var (
	fnInstallJS        = installJS
	fnInstallGo        = installGo
	fnInstallPy        = installPy

	fnRunGoTests       = runGoTests
	fnRunPyTests       = runPyTests

	fnTestWebMock      = testWebMock
	fnTestWebLiveHost  = testWebLiveHost

	fnHasHostModels    = hasHostModels
)
