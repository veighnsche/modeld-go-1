package manager

// cgo link directives for the in-process llama adapter.
// - We set an rpath of $ORIGIN so the runtime loader finds libllama.so and
//   libggml*.so in the same directory as the built Go binary (./bin).
// - We add -L${SRCDIR}/../../bin so the linker finds libllama.so at link time
//   when building the 'llama' variant.
// - No environment variables are required.
/*
#cgo LDFLAGS: -Wl,-rpath,'$ORIGIN' -L${SRCDIR}/../../bin -lllama
*/
import "C"
