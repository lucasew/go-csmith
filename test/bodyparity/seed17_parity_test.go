package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 17: StatementGoto forward-cond visible-read pool size (UP nOk=5 vs GO nOk=4)
// yields if (l_16) vs if (l_21). Body identical until that goto; first_div at
// ChooseOKVar after matched dest selection (map_accum_effect[other] / filter).
func TestSeed17(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 17
	assertOptsBodyParity(t, o)
}
