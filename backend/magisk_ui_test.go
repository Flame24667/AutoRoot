package main

import (
	"strconv"
	"testing"
)

func TestScreenAwakePreferencePreservesAllChargingSources(t *testing.T) {
	for want := 0; want <= 15; want++ {
		got, err := parseScreenAwakePreference("\r\n" + strconv.Itoa(want) + "\r\n")
		if err != nil || got != want {
			t.Fatalf("mask %d must be preserved unchanged: got %d, error %v", want, got, err)
		}
	}
}

func TestScreenAwakePreferenceRejectsUnknownOrMalformedValues(t *testing.T) {
	for _, raw := range []string{"", "null", "-1", "16", "255", "2147483648", "permission denied", "15\nwarning", "1.5"} {
		if _, err := parseScreenAwakePreference(raw); err == nil {
			t.Fatalf("invalid value %q was accepted", raw)
		}
	}
}

func TestPhoneUIRejectsBadOrAmbiguousControls(t *testing.T) {
	if _, err := parsePhoneUI("not XML"); err == nil {
		t.Fatal("non-XML accepted")
	}
	if _, _, err := nodeCenter(phoneUINode{Bounds: "[1,2][1,3]"}); err == nil {
		t.Fatal("empty bounds accepted")
	}
	n := phoneUINode{Package: "com.topjohnwu.magisk", Text: "Install", Enabled: "true", Bounds: "[480,364][660,454]"}
	if _, err := singleUINode([]phoneUINode{n, n}, n.Package, "Install"); err == nil {
		t.Fatal("ambiguous install accepted")
	}
	if _, err := singleUINode([]phoneUINode{n}, "com.android.systemui", "Install"); err == nil {
		t.Fatal("wrong app accepted")
	}
}

func TestMagiskInstallUsesMagiskCardNotAppCard(t *testing.T) {
	nodes := []phoneUINode{
		{ID: "com.topjohnwu.magisk:id/home_magisk_title", Bounds: "[180,387][480,431]"},
		{ID: "com.topjohnwu.magisk:id/home_manager_title", Bounds: "[180,664][480,708]"},
		{Package: "com.topjohnwu.magisk", Text: "Install", Clickable: "true", Enabled: "true", Bounds: "[480,364][660,454]"},
		{Package: "com.topjohnwu.magisk", Text: "Install", Clickable: "true", Enabled: "true", Bounds: "[480,641][660,731]"},
	}
	n, err := magiskInstallNode(nodes)
	if err != nil || n.Bounds != nodes[2].Bounds {
		t.Fatalf("wrong card: %+v %v", n, err)
	}
	x, y, err := nodeCenter(n)
	if err != nil || x != 570 || y != 409 {
		t.Fatal("incorrect node center")
	}
}

func TestPatchOutputRequiresCompletionAndExactFreshPath(t *testing.T) {
	nodes := []phoneUINode{{Package: magiskUIPackage, Text: "- Output file is written to /storage/emulated/0/Download/magisk_patched-30700_ABCD.tar"}}
	if _, err := patchOutputFromUI(nodes); err == nil {
		t.Fatal("partial log accepted")
	}
	nodes = append(nodes, phoneUINode{Package: magiskUIPackage, Text: "- All done!"})
	path, err := patchOutputFromUI(nodes)
	if err != nil || path != "/sdcard/Download/magisk_patched-30700_ABCD.tar" {
		t.Fatalf("unexpected output %s %v", path, err)
	}
	nodes = append(nodes, phoneUINode{Package: magiskUIPackage, Text: "/sdcard/Download/magisk_patched-30700_OTHER.tar"})
	if _, err := patchOutputFromUI(nodes); err == nil {
		t.Fatal("ambiguous output accepted")
	}
}

func TestRamdiskMustBeProvenOnRecognizedHome(t *testing.T) {
	nodes := []phoneUINode{{Package: magiskUIPackage, Text: "N/A"}, {Package: magiskUIPackage, Text: "Ramdisk", Enabled: "true", Bounds: "[90,531][180,558]"}, {Package: magiskUIPackage, Text: "Yes", Bounds: "[195,531][233,558]"}}
	if err := requireRamdiskHome(nodes); err != nil {
		t.Fatal(err)
	}
	nodes[2].Text = "No"
	if err := requireRamdiskHome(nodes); err == nil {
		t.Fatal("ramdisk No accepted")
	}
}
