package icon

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSolidPNG(t *testing.T, path string, size int, c color.RGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestToSafeVectorDrawableAddsGroupPadding(t *testing.T) {
	svg := &ParsedSVG{
		ViewportWidth:  24,
		ViewportHeight: 24,
		Paths: []ParsedPath{
			{Data: "M0,0h24v24h-24z", FillColor: "#FFFFFFFF"},
		},
	}
	xml := svg.ToSafeVectorDrawable(288, 288, 0.50)
	if !strings.Contains(xml, `android:viewportWidth="288.0"`) {
		t.Fatalf("expected 288.0 viewport, got:\n%s", xml)
	}
	if !strings.Contains(xml, `android:scaleX="6.0000"`) || !strings.Contains(xml, `android:translateX="72.0000"`) {
		t.Fatalf("expected centered 50%% group transform, got:\n%s", xml)
	}
}

func TestRunIconGeneratorOpaquePNGCleansDuplicateSplashXMLAndRoundsCorners(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "app", "src", "main", "res")
	if err := os.MkdirAll(filepath.Join(resDir, "drawable"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(resDir, "values"), 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-existing XML splash from jet init + manifest without roundIcon
	if err := os.WriteFile(filepath.Join(resDir, "drawable", "ic_splash_logo.xml"), []byte("<vector/>"), 0644); err != nil {
		t.Fatal(err)
	}
	themesXML := `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <style name="Theme.App.SplashScreen" parent="Theme.SplashScreen.IconBackground">
        <item name="windowSplashScreenBackground">@color/splash_background</item>
        <item name="windowSplashScreenAnimatedIcon">@drawable/ic_splash_logo</item>
        <item name="windowSplashScreenIconBackgroundColor">@color/ic_launcher_background</item>
    </style>
</resources>`
	if err := os.WriteFile(filepath.Join(resDir, "values", "themes.xml"), []byte(themesXML), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `<manifest><application android:icon="@android:drawable/sym_def_app_icon"></application></manifest>`
	if err := os.WriteFile(filepath.Join(root, "app", "src", "main", "AndroidManifest.xml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	logoPath := filepath.Join(root, "logo.png")
	writeSolidPNG(t, logoPath, 256, color.RGBA{R: 110, G: 108, B: 243, A: 255})

	err := RunIconGenerator(Options{
		InputPath:  logoPath,
		BgColor:    "#6E6CF3",
		ProjectDir: root,
		JSON:       false,
	})
	if err != nil {
		t.Fatalf("RunIconGenerator failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(resDir, "drawable", "ic_splash_logo.xml")); !os.IsNotExist(err) {
		t.Fatal("expected stale drawable/ic_splash_logo.xml to be removed")
	}
	splashPNG := filepath.Join(resDir, "drawable-nodpi", "ic_splash_logo.png")
	f, err := os.Open(splashPNG)
	if err != nil {
		t.Fatalf("expected drawable-nodpi/ic_splash_logo.png: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, cornerA := img.At(0, 0).RGBA()
	if cornerA != 0 {
		t.Fatalf("expected transparent outer corner on splash PNG, got alpha=%d", cornerA)
	}
	_, _, _, centerA := img.At(256, 256).RGBA()
	if centerA == 0 {
		t.Fatal("expected opaque center on splash PNG")
	}
	// Outer corner of the 52% inner square (~offset 123) should be rounded to transparent
	_, _, _, innerCornerA := img.At(123, 123).RGBA()
	if innerCornerA != 0 {
		t.Fatalf("expected rounded inner square corner to be transparent, got alpha=%d", innerCornerA)
	}

	updatedTheme, _ := os.ReadFile(filepath.Join(resDir, "values", "themes.xml"))
	if strings.Contains(string(updatedTheme), "Theme.SplashScreen.IconBackground") {
		t.Fatalf("expected Theme.SplashScreen without IconBackground for opaque squircle splash:\n%s", string(updatedTheme))
	}

	updatedManifest, _ := os.ReadFile(filepath.Join(root, "app", "src", "main", "AndroidManifest.xml"))
	if !strings.Contains(string(updatedManifest), `android:roundIcon="@mipmap/ic_launcher_round"`) {
		t.Fatalf("expected roundIcon added to manifest:\n%s", string(updatedManifest))
	}
}
