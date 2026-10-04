package cmd

import (
	"os"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

func TestMain(m *testing.M) { os.Exit(globaldir.Main(m)) }
