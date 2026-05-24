package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// pageHandler returns a server route that produces synthetic pages of size
// pageSize, totalling totalItems, where each item is {"id": N}.
func pageHandler(pageSize, totalItems int) routeHandler {
	return func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.Header().Set("Content-Type", "application/json")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		ps, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		if ps <= 0 {
			ps = pageSize
		}
		start := (page - 1) * ps
		if start >= totalItems {
			w.Write([]byte(`{"items":[]}`))
			return
		}
		end := start + ps
		if end > totalItems {
			end = totalItems
		}
		items := make([]map[string]int, 0, end-start)
		for i := start; i < end; i++ {
			items = append(items, map[string]int{"id": i + 1})
		}
		body, _ := json.Marshal(map[string]any{"items": items, "total": totalItems})
		w.Write(body)
	}
}

func TestPaginate_DefaultSinglePage(t *testing.T) {
	srv, st := newTestServer(t)
	st.route("GET", "/apiv2/p", pageHandler(10, 25))
	c, _ := newAuthedTestClient(t, srv)
	pf := &pageFlags{PageSize: 10}
	items, _, err := paginate(c, "/apiv2/p", url.Values{}, pf)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 10 {
		t.Errorf("expected 10 items (default single page), got %d", len(items))
	}
	if len(st.requestsTo("GET", "/apiv2/p")) != 1 {
		t.Errorf("expected 1 network call, got %d", len(st.requestsTo("GET", "/apiv2/p")))
	}
}

func TestPaginate_AllDrainsPages(t *testing.T) {
	srv, st := newTestServer(t)
	st.route("GET", "/apiv2/p", pageHandler(10, 25))
	c, _ := newAuthedTestClient(t, srv)
	pf := &pageFlags{All: true, PageSize: 10}
	items, _, err := paginate(c, "/apiv2/p", url.Values{}, pf)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 25 {
		t.Errorf("expected 25 items, got %d", len(items))
	}
	// 3 pages: 10 + 10 + 5 (short page terminates)
	if got := len(st.requestsTo("GET", "/apiv2/p")); got != 3 {
		t.Errorf("expected 3 page fetches, got %d", got)
	}
}

func TestPaginate_LimitCapsTotal(t *testing.T) {
	srv, st := newTestServer(t)
	_ = st
	st.route("GET", "/apiv2/p", pageHandler(10, 100))
	c, _ := newAuthedTestClient(t, srv)
	pf := &pageFlags{All: true, PageSize: 10, Limit: 15}
	items, _, err := paginate(c, "/apiv2/p", url.Values{}, pf)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 15 {
		t.Errorf("expected 15 items (limit), got %d", len(items))
	}
}

func TestPaginate_RecognisesPlainArray(t *testing.T) {
	srv, st := newTestServer(t)
	st.route("GET", "/apiv2/p", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1},{"id":2}]`))
	})
	c, _ := newAuthedTestClient(t, srv)
	items, _, err := paginate(c, "/apiv2/p", url.Values{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items from plain array, got %d", len(items))
	}
}

func TestPaginate_UnknownShapeReturnsFirstPageRaw(t *testing.T) {
	srv, st := newTestServer(t)
	st.route("GET", "/apiv2/p", func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"weird":"shape"}`))
	})
	c, _ := newAuthedTestClient(t, srv)
	items, firstPage, err := paginate(c, "/apiv2/p", url.Values{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if items != nil {
		t.Errorf("expected nil items for unrecognised shape, got %v", items)
	}
	if !strings.Contains(string(firstPage), "weird") {
		t.Errorf("expected raw first page returned, got %q", firstPage)
	}
}

func TestCmd_AbsenceList_PagesAll(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.route("GET", "/apiv2/absences/manager", pageHandler(50, 120))
	if err := runCmd(t, "absence", "list", "--all"); err != nil {
		t.Fatal(err)
	}
	// 50 + 50 + 20 -> 3 fetches
	if got := len(st.requestsTo("GET", "/apiv2/absences/manager")); got != 3 {
		t.Errorf("expected 3 fetches, got %d", got)
	}
}

func TestCmd_ExpenseList_Limit(t *testing.T) {
	srv, st, _, buf := withTestEnv(t)
	_ = srv
	st.route("GET", "/apiv2/expense/user/paginated", pageHandler(50, 200))
	if err := runCmd(t, "expense", "list", "--all", "--limit", "7"); err != nil {
		t.Fatal(err)
	}
	// 7 items should appear in the JSON output
	var out []map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &out); err != nil {
		t.Fatalf("output not JSON: %s", buf.String())
	}
	if len(out) != 7 {
		t.Errorf("expected 7 items capped by --limit, got %d", len(out))
	}
}

