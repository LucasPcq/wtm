package cli

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/infra/proc"
)

func TestASecondInterruptWaitsForAShieldedStep(t *testing.T) {
	if os.Getenv(shieldHelperEnv) != "" {
		ctx, stop := interruptible()
		defer stop()
		_, release := proc.Shield(ctx)
		fmt.Println("armed")
		<-ctx.Done()
		fmt.Println("cancelled")
		time.Sleep(time.Second)
		fmt.Println("released")
		release()
		time.Sleep(time.Minute)
		os.Exit(0)
	}

	helper := startHelper(t, "TestASecondInterruptWaitsForAShieldedStep")
	helper.expect(t, "armed")
	_ = helper.cmd.Process.Signal(os.Interrupt)
	helper.expect(t, "cancelled")
	_ = helper.cmd.Process.Signal(os.Interrupt)
	helper.expect(t, "released")
	helper.diesOnSIGINT(t)
}

func TestASecondInterruptTakesTheChildItWasStoppingWithIt(t *testing.T) {
	if os.Getenv(shieldHelperEnv) != "" {
		ctx, stop := interruptible()
		defer stop()
		child := proc.Command(ctx, "sh", "-c", `trap '' INT TERM; echo $$; exec sleep 30`)
		child.Stdout = os.Stdout
		_ = child.Run()
		time.Sleep(time.Minute)
		os.Exit(0)
	}

	helper := startHelper(t, "TestASecondInterruptTakesTheChildItWasStoppingWithIt")
	pid, err := strconv.Atoi(helper.expect(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	time.Sleep(100 * time.Millisecond)
	_ = helper.cmd.Process.Signal(os.Interrupt)
	time.Sleep(100 * time.Millisecond)
	_ = helper.cmd.Process.Signal(os.Interrupt)
	helper.diesOnSIGINT(t)

	deadline := time.Now().Add(500 * time.Millisecond)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatal("the child that ignored the interrupt outlived the run")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
