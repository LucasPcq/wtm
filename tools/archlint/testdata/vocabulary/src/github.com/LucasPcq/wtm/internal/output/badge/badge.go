package badge

import st "github.com/LucasPcq/wtm/internal/styles"

var chip = st.BadgeOK // want `internal/output must not use styles\.BadgeOK`

var title = st.DashboardTitle // want `internal/output must not use styles\.DashboardTitle`

var fine = st.Accent
