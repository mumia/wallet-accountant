//go:build integration

package http_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

type mapping = map[string]any

type connection struct {
	ConnectionID      string  `json:"connection_id"`
	AccountID         string  `json:"account_id"`
	Kind              string  `json:"kind"`
	CSVMapping        mapping `json:"csv_mapping"`
	CSVMappingVersion int     `json:"csv_mapping_version"`
}

type connectionList struct {
	Items []connection `json:"items"`
}

// signed maps a headed file with one signed amount column.
func signed() mapping {
	return mapping{
		"delimiter":             ";",
		"has_header_row":        true,
		"skip_leading_lines":    0,
		"date_format":           "DD/MM/YYYY",
		"decimal_separator":     ",",
		"thousands_separator":   ".",
		"amount_layout":         "SIGNED",
		"positive_amount_means": "MONEY_IN",
		"columns": map[string]any{
			"booking_date": map[string]any{"header": "Date"},
			"description":  map[string]any{"header": "Description"},
			"amount":       map[string]any{"header": "Amount"},
		},
	}
}

// debitCredit maps a headerless file with debit and credit columns, by
// position, with every optional field.
func debitCredit() mapping {
	return mapping{
		"delimiter":           ",",
		"has_header_row":      false,
		"skip_leading_lines":  2,
		"date_format":         "YYYY-MM-DD",
		"decimal_separator":   ".",
		"thousands_separator": "",
		"amount_layout":       "DEBIT_CREDIT",
		"columns": map[string]any{
			"booking_date":        map[string]any{"position": 1},
			"description":         map[string]any{"position": 2},
			"debit":               map[string]any{"position": 3},
			"credit":              map[string]any{"position": 4},
			"transaction_date":    map[string]any{"position": 5},
			"value_date":          map[string]any{"position": 6},
			"payee":               map[string]any{"position": 7},
			"bank_transaction_id": map[string]any{"position": 8},
		},
	}
}

// withColumns returns m with some columns replaced (nil removes one).
func withColumns(m mapping, columns map[string]any) mapping {
	out := maps.Clone(m)
	cols := maps.Clone(m["columns"].(map[string]any))

	for k, v := range columns {
		if v == nil {
			delete(cols, k)

			continue
		}

		cols[k] = v
	}

	out["columns"] = cols

	return out
}

func with(m mapping, fields mapping) mapping {
	out := maps.Clone(m)

	for k, v := range fields {
		if v == nil {
			delete(out, k)

			continue
		}

		out[k] = v
	}

	return out
}

// sameJSON compares two values as JSON, so numbers and maps compare by value.
func sameJSON(t *testing.T, got, want any) bool {
	t.Helper()

	norm := func(v any) any {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}

		var out any
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}

		return out
	}

	return reflect.DeepEqual(norm(got), norm(want))
}

type fixture struct {
	app       *apitest.App
	token     string
	household string
	account   string
}

func (f fixture) connections() string {
	return "/households/" + f.household + "/accounts/" + f.account + "/connections"
}

func (f fixture) mappingOf(c connection) string {
	return f.connections() + "/" + c.ConnectionID + "/csv-mapping"
}

func (f fixture) create(t *testing.T, body map[string]any) connection {
	t.Helper()

	var created connection
	f.app.Post(t, f.connections(), f.token, body).AssertJSON(t, http.StatusCreated, &created)

	return created
}

func (f fixture) list(t *testing.T) []connection {
	t.Helper()

	var got connectionList
	f.app.Get(t, f.connections(), f.token).AssertJSON(t, http.StatusOK, &got)

	return got.Items
}

func newFixture(t *testing.T, app *apitest.App, subject string) fixture {
	t.Helper()

	token := app.Register(t, "https://issuer.example", subject)

	var household struct {
		HouseholdID string `json:"household_id"`
	}

	app.Post(t, "/households", token, map[string]string{"name": "Home", "base_currency": "EUR"}).
		AssertJSON(t, http.StatusCreated, &household)

	return fixture{app: app, token: token, household: household.HouseholdID, account: addAccount(t, app, token, household.HouseholdID)}
}

func addAccount(t *testing.T, app *apitest.App, token, household string) string {
	t.Helper()

	var account struct {
		AccountID string `json:"account_id"`
	}

	app.Post(t, "/households/"+household+"/accounts", token, map[string]string{
		"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
	}).AssertJSON(t, http.StatusCreated, &account)

	return account.AccountID
}

func TestCreatingAConnectionAndListingIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")

	if got := f.list(t); len(got) != 0 {
		t.Fatalf("before creating any: %+v, want none", got)
	}

	created := f.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT"})

	if created.ConnectionID == "" || created.AccountID != f.account || created.Kind != "STATEMENT_FILE_IMPORT" ||
		created.CSVMapping != nil || created.CSVMappingVersion != 0 {
		t.Fatalf("created = %+v, want a statement file import of the Account with no mapping yet", created)
	}

	if got := f.list(t); len(got) != 1 || !reflect.DeepEqual(got[0], created) {
		t.Errorf("GET connections = %+v, want only %+v", got, created)
	}
}

func TestCreatingAConnectionWithItsMapping(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")

	created := f.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT", "csv_mapping": signed()})

	if !sameJSON(t, created.CSVMapping, signed()) || created.CSVMappingVersion != 1 {
		t.Errorf("created = %+v, want the signed mapping at version 1", created)
	}
}

func TestSavingEveryKindOfMapping(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")
	c := f.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT"})

	for name, m := range map[string]mapping{
		"signed by header":       signed(),
		"inverted sign":          with(signed(), mapping{"positive_amount_means": "MONEY_OUT"}),
		"debit and credit":       debitCredit(),
		"positional with header": withColumns(signed(), map[string]any{"amount": map[string]any{"position": 3}}),
		"tab and compact dates": with(signed(), mapping{
			"delimiter": "\t", "date_format": "YYYYMMDD", "decimal_separator": ".", "thousands_separator": "'",
		}),
	} {
		t.Run(name, func(t *testing.T) {
			var saved connection
			f.app.Put(t, f.mappingOf(c), f.token, m).AssertJSON(t, http.StatusOK, &saved)

			if !sameJSON(t, saved.CSVMapping, m) || saved.ConnectionID != c.ConnectionID {
				t.Errorf("saved = %+v, want mapping %+v", saved.CSVMapping, m)
			}

			got := f.list(t)
			if len(got) != 1 || !sameJSON(t, got[0].CSVMapping, m) || got[0].CSVMappingVersion != saved.CSVMappingVersion {
				t.Errorf("GET connections = %+v, want the saved mapping", got)
			}
		})
	}
}

func TestEverySaveChangesTheMappingVersion(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")
	c := f.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT"})

	versions := make([]int, 0, 3)

	for _, m := range []mapping{signed(), debitCredit(), debitCredit()} {
		var saved connection
		f.app.Put(t, f.mappingOf(c), f.token, m).AssertJSON(t, http.StatusOK, &saved)
		versions = append(versions, saved.CSVMappingVersion)
	}

	if !reflect.DeepEqual(versions, []int{1, 2, 3}) {
		t.Errorf("versions = %v, want a new one on every save, even of the same mapping", versions)
	}
}

func TestInvalidMappingsAreRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")
	c := f.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT", "csv_mapping": signed()})

	for name, tc := range map[string]struct {
		mapping mapping
		detail  string
	}{
		"no Booking date":     {withColumns(signed(), map[string]any{"booking_date": nil}), "Booking date"},
		"no amount column":    {withColumns(signed(), map[string]any{"amount": nil}), "amount"},
		"no credit column":    {withColumns(debitCredit(), map[string]any{"credit": nil}), "credit"},
		"no amount layout":    {with(signed(), mapping{"amount_layout": nil}), "amount layout"},
		"no sign":             {with(signed(), mapping{"positive_amount_means": nil}), "positive"},
		"unknown date format": {with(signed(), mapping{"date_format": "dd/mm/yyyy"}), "date format"},
		"same column twice": {
			withColumns(signed(), map[string]any{"payee": map[string]any{"header": "amount"}}), "both read from",
		},
		"same position twice": {
			withColumns(debitCredit(), map[string]any{"payee": map[string]any{"position": 1}}), "both read from",
		},
		"header without a header row": {
			withColumns(debitCredit(), map[string]any{"payee": map[string]any{"header": "Payee"}}), "no header row",
		},
		"same separators": {with(signed(), mapping{"thousands_separator": ","}), "thousands separator"},
		"long delimiter":  {with(signed(), mapping{"delimiter": ";;"}), "delimiter"},
		"empty body":      {mapping{}, "Booking date"},
	} {
		t.Run(name, func(t *testing.T) {
			p := f.app.Put(t, f.mappingOf(c), f.token, tc.mapping).
				AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-csv-mapping")

			if !strings.Contains(p.Detail, tc.detail) {
				t.Errorf("detail = %q, want it to mention %q", p.Detail, tc.detail)
			}
		})
	}

	t.Run("every problem at once", func(t *testing.T) {
		m := with(withColumns(signed(), map[string]any{
			"booking_date": nil,
			"payee":        map[string]any{"header": "Description"},
		}), mapping{"date_format": "whenever"})

		p := f.app.Put(t, f.mappingOf(c), f.token, m).
			AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-csv-mapping")

		for _, want := range []string{"Booking date", "date format", "both read from"} {
			if !strings.Contains(p.Detail, want) {
				t.Errorf("detail = %q, want it to mention %q", p.Detail, want)
			}
		}
	})

	got := f.list(t)
	if len(got) != 1 || !sameJSON(t, got[0].CSVMapping, signed()) || got[0].CSVMappingVersion != 1 {
		t.Errorf("after refusals: %+v, want the first mapping unchanged at version 1", got)
	}
}

func TestCreatingAConnectionIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")

	for name, body := range map[string]map[string]any{
		"bank feed":       {"kind": "BANK_FEED"},
		"unknown kind":    {"kind": "CARRIER_PIGEON"},
		"lower-case kind": {"kind": "statement_file_import"},
	} {
		t.Run(name, func(t *testing.T) {
			f.app.Post(t, f.connections(), f.token, body).
				AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-connection")
		})
	}

	t.Run("invalid mapping", func(t *testing.T) {
		f.app.Post(t, f.connections(), f.token, map[string]any{
			"kind": "STATEMENT_FILE_IMPORT", "csv_mapping": with(signed(), mapping{"date_format": "soon"}),
		}).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-csv-mapping")
	})

	if got := f.list(t); len(got) != 0 {
		t.Errorf("after refusals: %+v, want no Connection", got)
	}
}

func TestAnAccountListsOnlyItsOwnConnections(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")
	other := f
	other.account = addAccount(t, f.app, f.token, f.household)

	first := f.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT"})
	second := f.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT", "csv_mapping": debitCredit()})
	other.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT"})

	got := f.list(t)
	if len(got) != 2 || got[0].ConnectionID != first.ConnectionID || got[1].ConnectionID != second.ConnectionID {
		t.Errorf("GET connections = %+v, want %s then %s", got, first.ConnectionID, second.ConnectionID)
	}

	t.Run("saving through another Account", func(t *testing.T) {
		f.app.Put(t, other.mappingOf(first), f.token, signed()).AssertProblem(t, http.StatusNotFound, "about:blank")
	})
}

func TestConnectionsOfAnUnknownAccountAre404(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")
	unknown := f
	unknown.account = f.household

	unknown.app.Get(t, unknown.connections(), f.token).AssertProblem(t, http.StatusNotFound, "about:blank")
	unknown.app.Post(t, unknown.connections(), f.token, map[string]any{"kind": "STATEMENT_FILE_IMPORT"}).
		AssertProblem(t, http.StatusNotFound, "about:blank")
	f.app.Put(t, f.connections()+"/"+f.account+"/csv-mapping", f.token, signed()).
		AssertProblem(t, http.StatusNotFound, "about:blank")
}

func TestAConnectionSurvivesARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newFixture(t, app, "alice")
	c := f.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT", "csv_mapping": signed()})

	var saved connection
	f.app.Put(t, f.mappingOf(c), f.token, debitCredit()).AssertJSON(t, http.StatusOK, &saved)

	app.Rebuild(t)

	if got := f.list(t); len(got) != 1 || !reflect.DeepEqual(got[0], saved) {
		t.Errorf("after rebuild = %+v, want %+v", got, saved)
	}
}

func TestCreatingAConnectionTwiceWithOneIdempotencyKeyCreatesOne(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")
	body := map[string]any{"kind": "STATEMENT_FILE_IMPORT", "csv_mapping": signed()}

	var first, replay connection
	f.app.PostWithKey(t, f.connections(), f.token, "k1", body).AssertJSON(t, http.StatusCreated, &first)
	f.app.PostWithKey(t, f.connections(), f.token, "k1", body).AssertJSON(t, http.StatusCreated, &replay)

	if !reflect.DeepEqual(replay, first) {
		t.Errorf("replay = %+v, want %+v", replay, first)
	}

	if got := f.list(t); len(got) != 1 {
		t.Errorf("connections = %+v, want only the first", got)
	}
}

func TestAnotherHouseholdsConnectionsAre404(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := newFixture(t, app, "alice")
	bob := newFixture(t, app, "bob")
	c := alice.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT", "csv_mapping": signed()})

	asBob := alice
	asBob.token = bob.token

	// Alice's Account and Connection through Bob's own Household.
	throughBobs := alice
	throughBobs.token, throughBobs.household = bob.token, bob.household

	for name, f := range map[string]fixture{"in Alice's Household": asBob, "through Bob's Household": throughBobs} {
		t.Run(name, func(t *testing.T) {
			app.Get(t, f.connections(), f.token).AssertProblem(t, http.StatusNotFound, "about:blank")
			app.Post(t, f.connections(), f.token, map[string]any{"kind": "STATEMENT_FILE_IMPORT"}).
				AssertProblem(t, http.StatusNotFound, "about:blank")
			app.Put(t, f.mappingOf(c), f.token, debitCredit()).AssertProblem(t, http.StatusNotFound, "about:blank")
		})
	}

	t.Run("Bob's Account with Alice's Connection", func(t *testing.T) {
		app.Put(t, bob.mappingOf(c), bob.token, debitCredit()).AssertProblem(t, http.StatusNotFound, "about:blank")
	})

	got := alice.list(t)
	if len(got) != 1 || !sameJSON(t, got[0].CSVMapping, signed()) || got[0].CSVMappingVersion != 1 {
		t.Errorf("Alice's connections = %+v, want hers untouched", got)
	}
}
