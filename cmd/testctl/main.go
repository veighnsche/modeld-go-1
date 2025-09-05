package main

// Legacy monolithic implementation has been modularized across files:
// - cli.go       (Config, usage, parseConfig, run, main)
// - install.go   (installJS, installGo, installPy)
// - tests.go     (runGoTests, runPyTests)
// - web.go       (testWebMock, testWebLiveHost)
// - ports.go     (chooseFreePort, isPortBusy, waitHTTP, ensurePorts)
// - fs.go        (firstGGUF, hasHostModels, homeDir)
// - executil.go  (runCmdVerbose, runCmdStreaming, runEnvCmdStreaming, stream, ioReader)
// - logenv.go    (info, warn, die, must, envStr, envBool, envInt)
