package codecconfig

import "testing"

func TestDefaultLiteralCodecEffectiveModeFallback(t *testing.T) {
	previousMode := DefaultLiteralCodecModeValue()
	defer func() {
		_ = SetDefaultLiteralCodecMode(previousMode)
	}()

	if err := SetDefaultLiteralCodecMode(LiteralCodecModeStrict); err != nil {
		t.Fatalf("SetDefaultLiteralCodecMode() error = %v", err)
	}

	if got := (DefaultLiteralCodec{}).effectiveMode(); got != LiteralCodecModeStrict {
		t.Fatalf("unexpected effective mode: %q", got)
	}

	if got := (DefaultLiteralCodec{Mode: "broken"}).effectiveMode(); got != LiteralCodecModeStrict {
		t.Fatalf("unexpected fallback effective mode: %q", got)
	}
}
