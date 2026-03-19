package missav

import (
	"testing"

	"github.com/metatube-community/metatube-sdk-go/provider/internal/testkit"
)

func TestMissAV_GetActorInfoByID(t *testing.T) {
	testkit.Test(t, New, []string{
		"三上悠亞",
		"美竹鈴",
		"宇流木さら",
	})
}

func TestMissAV_SearchActor(t *testing.T) {
	testkit.Test(t, New, []string{
		"三上悠亚",
		"美竹すず",
		"宇流木さら",
	})
}

func TestMissAV_GetMovieInfoByID(t *testing.T) {
	testkit.Test(t, New, []string{
		"OFJE-550",
		"SSIS-834",
	})
}
