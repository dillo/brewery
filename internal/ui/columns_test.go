package ui

import "testing"

func TestColumnsFor(t *testing.T) {
	for _, tt := range []struct {
		w          int
		focus      column
		hasDetails bool
		want       columns
	}{
		// ≥122: all three, whatever has the focus
		{140, colSidebar, true, columns{22, 64, 54}},
		{140, colList, true, columns{22, 64, 54}},
		{140, colDetails, true, columns{22, 64, 54}},
		{122, colList, true, columns{22, 55, 45}},
		// 100–121: the sidebar and list, or the list and details
		{121, colSidebar, true, columns{22, 99, 0}},
		{121, colList, true, columns{0, 66, 55}},
		{121, colDetails, true, columns{0, 66, 55}},
		{100, colList, true, columns{0, 55, 45}},
		{100, colSidebar, true, columns{22, 78, 0}},
		// 82–99: the sidebar and list, or the details alone
		{99, colSidebar, true, columns{22, 77, 0}},
		{99, colList, true, columns{22, 77, 0}},
		{99, colDetails, true, columns{0, 0, 99}},
		{82, colList, true, columns{22, 60, 0}},
		// <82: the focused column alone, at full width
		{81, colSidebar, true, columns{81, 0, 0}},
		{81, colList, true, columns{0, 81, 0}},
		{81, colDetails, true, columns{0, 0, 81}},
		// no details (Home, Outdated, Taps, jobs): sidebar and list, or one
		{140, colList, false, columns{22, 118, 0}},
		{140, colDetails, false, columns{22, 118, 0}}, // a details focus is the list's
		{82, colSidebar, false, columns{22, 60, 0}},
		{81, colSidebar, false, columns{81, 0, 0}},
		{81, colList, false, columns{0, 81, 0}},
	} {
		if got := columnsFor(tt.w, tt.focus, tt.hasDetails); got != tt.want {
			t.Errorf("columnsFor(%d, %d, %v) = %+v, want %+v", tt.w, tt.focus, tt.hasDetails, got, tt.want)
		}
	}
}
