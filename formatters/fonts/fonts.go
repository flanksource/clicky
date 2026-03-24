package fonts

import (
	_ "embed"

	"github.com/carlos7ags/folio/font"
)

//go:embed DejaVuSansMono.ttf
var dejaVuSansMonoTTF []byte

var MonoFont *font.EmbeddedFont

func init() {
	face, err := font.ParseFont(dejaVuSansMonoTTF)
	if err != nil {
		panic("fonts: failed to parse DejaVuSansMono.ttf: " + err.Error())
	}
	MonoFont = font.NewEmbeddedFont(face)
}
