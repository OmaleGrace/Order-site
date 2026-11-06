package safe

import "testing"

// A panic inside a background job must be swallowed. If Recover did not work, the panic would
// end the whole test program, so reaching the end of this test is the proof.
func TestRecoverStopsAPanicFromEndingTheProgram(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer Recover("test job")
		panic("boom")
	}()
	<-done
}

func TestRecoverDoesNothingWhenThereIsNoPanic(t *testing.T) {
	defer Recover("quiet job")
}
