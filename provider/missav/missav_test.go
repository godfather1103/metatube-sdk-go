package missav

import (
	"fmt"
	"testing"

	"github.com/metatube-community/metatube-sdk-go/provider/internal/testkit"
)

// Deprecated: 此方法将在未来版本中删除
func TestMissAV_GetActorInfoByID(t *testing.T) {
	miss := New()
	id, err := miss.getActorInfoByID("三上悠亞")
	if err != nil {
		return
	}
	fmt.Println(id)
	fmt.Println(miss.parseActorIDFromURL(id.Homepage))
	testkit.Test(t, New, []string{
		"三上悠亞",
		"美竹鈴",
		"宇流木さら",
	})
}

// Deprecated: 此方法将在未来版本中删除
func TestMissAV_SearchActor(t *testing.T) {
	id, err := New().searchActor("三上悠亞")
	if err != nil {
		return
	}
	fmt.Println(id)
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
