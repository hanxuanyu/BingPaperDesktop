package bing

import "testing"

func TestNormalizeOfficialImageURL_RemovesTailParamsAfterJPG(t *testing.T) {
	raw := "https://www.bing.com/th?id=OHR.SamuiThailand_ZH-CN3323790951_UHD.jpg&rf=LaDigue_UHD.jpg&pid=hp&w=1920&h=1080&rs=1&c=4"
	got := normalizeOfficialImageURL(raw)
	want := "https://www.bing.com/th?id=OHR.SamuiThailand_ZH-CN3323790951_UHD.jpg"
	if got != want {
		t.Fatalf("unexpected normalized url:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestNormalizeOfficialImageURL_KeepWhenNoTailParams(t *testing.T) {
	raw := "https://www.bing.com/th?id=OHR.SamuiThailand_ZH-CN3323790951_UHD.jpg"
	got := normalizeOfficialImageURL(raw)
	if got != raw {
		t.Fatalf("url should stay unchanged:\nwant: %s\ngot:  %s", raw, got)
	}
}
