package doctor

import "testing"

func TestNextStep(t *testing.T) {
	cases := []struct {
		name   string
		report DoctorReport
		want   string
	}{
		{"nothing missing", DoctorReport{}, "run jet doctor again"},
		{"all checks pass", DoctorReport{Results: []CheckResult{
			{Name: "Java / JDK", Found: true},
			{Name: "Android SDK", Found: true},
		}}, "run jet doctor again"},
		{"failed check", DoctorReport{Results: []CheckResult{
			{Name: "SDK Platforms", Found: false, Detail: `Install via: sdkmanager "platforms;android-36" (or android-37)`},
		}}, `Install via: sdkmanager "platforms;android-36" (or android-37)`},
		{"warning check", DoctorReport{Results: []CheckResult{
			{Name: "Java / JDK", Found: true, Warning: true, Detail: "Please upgrade your JDK."},
		}}, "Please upgrade your JDK."},
		{"failed check without detail", DoctorReport{Results: []CheckResult{
			{Name: "Build Tools", Found: false},
		}}, "Build Tools"},
	}
	for _, c := range cases {
		if got := nextStep(&c.report); got != c.want {
			t.Errorf("%s: nextStep = %q, want %q", c.name, got, c.want)
		}
	}
}
