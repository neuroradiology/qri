package base

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qri-io/cafs"
	"github.com/qri-io/dataset"
	"github.com/qri-io/dataset/dstest"
	"github.com/qri-io/qri/repo"
	"github.com/qri-io/qri/repo/profile"
)

func TestListDatasets(t *testing.T) {
	r := newTestRepo(t)
	ref := addCitiesDataset(t, r)

	res, err := ListDatasets(r, 1, 0, false, false)
	if err != nil {
		t.Error(err.Error())
	}
	if len(res) != 1 {
		t.Error("expected one dataset response")
	}

	res, err = ListDatasets(r, 1, 0, false, true)
	if err != nil {
		t.Error(err.Error())
	}

	if len(res) != 0 {
		t.Error("expected no published datasets")
	}

	if err := SetPublishStatus(r, &ref, true); err != nil {
		t.Fatal(err)
	}

	res, err = ListDatasets(r, 1, 0, false, true)
	if err != nil {
		t.Error(err.Error())
	}

	if len(res) != 1 {
		t.Error("expected one published dataset response")
	}
}

func TestCreateDataset(t *testing.T) {
	r, err := repo.NewMemRepo(testPeerProfile, cafs.NewMapstore(), profile.NewMemStore(), nil)
	if err != nil {
		t.Fatal(err.Error())
	}

	ds := &dataset.Dataset{
		Meta:   &dataset.Meta{Title: "test"},
		Commit: &dataset.Commit{Title: "hello"},
		Structure: &dataset.Structure{
			Format: dataset.JSONDataFormat,
			Schema: dataset.BaseSchemaArray,
		},
	}

	if _, err := CreateDataset(r, "foo", &dataset.Dataset{}, nil, true); err == nil {
		t.Error("expected bad dataset to error")
	}

	ref, err := CreateDataset(r, "foo", ds, cafs.NewMemfileBytes("body.json", []byte("[]")), true)
	if err != nil {
		t.Fatal(err.Error())
	}
	refs, err := r.References(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Errorf("ref length mismatch. expected 1, got: %d", len(refs))
	}

	ds.Meta.Title = "an update"
	ds.PreviousPath = ref.Path

	ref, err = CreateDataset(r, "foo", ds, cafs.NewMemfileBytes("body.json", []byte("[]")), true)
	if err != nil {
		t.Fatal(err.Error())
	}
	refs, err = r.References(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Errorf("ref length mismatch. expected 1, got: %d", len(refs))
	}
}

func TestDatasetPodBodyFile(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"json":"data"}`))
	}))
	badS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	cases := []struct {
		dsp      *dataset.DatasetPod
		filename string
		fileLen  int
		err      string
	}{
		// bad input
		{&dataset.DatasetPod{}, "", 0, "not found"},

		// inline data
		{&dataset.DatasetPod{BodyBytes: []byte("a,b,c\n1,2,3")}, "", 0, "specifying bodyBytes requires format be specified in dataset.structure"},
		{&dataset.DatasetPod{Structure: &dataset.StructurePod{Format: "csv"}, BodyBytes: []byte("a,b,c\n1,2,3")}, "body.csv", 11, ""},

		// urlz
		{&dataset.DatasetPod{BodyPath: "http://"}, "", 0, "fetching body url: Get http:: http: no Host in request URL"},
		{&dataset.DatasetPod{BodyPath: fmt.Sprintf("%s/foobar.json", badS.URL)}, "", 0, "invalid status code fetching body url: 500"},
		{&dataset.DatasetPod{BodyPath: fmt.Sprintf("%s/foobar.json", s.URL)}, "foobar.json", 15, ""},

		// local filepaths
		{&dataset.DatasetPod{BodyPath: "nope.cbor"}, "", 0, "reading body file: open nope.cbor: no such file or directory"},
		{&dataset.DatasetPod{BodyPath: "nope.yaml"}, "", 0, "reading body file: open nope.yaml: no such file or directory"},
		{&dataset.DatasetPod{BodyPath: "testdata/schools.cbor"}, "schools.cbor", 154, ""},
		{&dataset.DatasetPod{BodyPath: "testdata/bad.yaml"}, "", 0, "converting yaml body to json: yaml: line 1: did not find expected '-' indicator"},
		{&dataset.DatasetPod{BodyPath: "testdata/oh_hai.yaml"}, "oh_hai.json", 29, ""},
	}

	for i, c := range cases {
		file, err := DatasetPodBodyFile(c.dsp)
		if !(err == nil && c.err == "" || err != nil && err.Error() == c.err) {
			t.Errorf("case %d error mismatch. expected: '%s', got: '%s'", i, c.err, err)
			continue
		}

		if file == nil && c.filename != "" {
			t.Errorf("case %d expected file", i)
			continue
		} else if c.filename == "" {
			continue
		}

		if c.filename != file.FileName() {
			t.Errorf("case %d filename mismatch. expected: '%s', got: '%s'", i, c.filename, file.FileName())
		}
		data, err := ioutil.ReadAll(file)
		if err != nil {
			t.Errorf("case %d error reading file: %s", i, err.Error())
			continue
		}
		if c.fileLen != len(data) {
			t.Errorf("case %d file length mismatch. expected: %d, got: %d", i, c.fileLen, len(data))
		}

		if err := file.Close(); err != nil {
			t.Errorf("case %d error closing file: %s", i, err.Error())
		}
	}
}

// func TestDataset(t *testing.T) {
// 	rc, _ := mock.NewMockServer()

// 	rmf := func(t *testing.T) repo.Repo {
// 		mr, err := repo.NewMemRepo(testPeerProfile, cafs.NewMapstore(), profile.NewMemStore(), rc)
// 		if err != nil {
// 			panic(err)
// 		}
// 		// mr.SetPrivateKey(privKey)
// 		return mr
// 	}
// 	DatasetTests(t, rmf)
// }

// func TestSaveDataset(t *testing.T) {
// 	n := newTestNode(t)

// 	// test Dry run
// 	ds := &dataset.Dataset{
// 		Commit:    &dataset.Commit{},
// 		Structure: &dataset.Structure{Format: dataset.JSONDataFormat, Schema: dataset.BaseSchemaArray},
// 		Meta: &dataset.Meta{
// 			Title: "test title",
// 		},
// 	}
// 	body := cafs.NewMemfileBytes("data.json", []byte("[]"))
// 	ref, _, err := SaveDataset(n, "dry_run_test", ds, body, nil, true, false)
// 	if err != nil {
// 		t.Errorf("dry run error: %s", err.Error())
// 	}
// 	if ref.AliasString() != "peer/dry_run_test" {
// 		t.Errorf("ref alias mismatch. expected: '%s' got: '%s'", "peer/dry_run_test", ref.AliasString())
// 	}
// }

// type RepoMakerFunc func(t *testing.T) repo.Repo
// type RepoTestFunc func(t *testing.T, rmf RepoMakerFunc)

// func DatasetTests(t *testing.T, rmf RepoMakerFunc) {
// 	for _, test := range []RepoTestFunc{
// 		testSaveDataset,
// 		testReadDataset,
// 		testRenameDataset,
// 		testDatasetPinning,
// 		testDeleteDataset,
// 		testEventsLog,
// 	} {
// 		test(t, rmf)
// 	}
// }

// func testSaveDataset(t *testing.T, rmf RepoMakerFunc) {
// 	createDataset(t, rmf)
// }

// func TestCreateDataset(t *testing.T, rmf RepoMakerFunc) (*p2p.QriNode, repo.DatasetRef) {
// 	r := rmf(t)
// 	r.SetProfile(testPeerProfile)
// 	n, err := p2p.NewQriNode(r, config.DefaultP2PForTesting())
// 	if err != nil {
// 		t.Error(err.Error())
// 		return n, repo.DatasetRef{}
// 	}

// 	tc, err := dstest.NewTestCaseFromDir(testdataPath("cities"))
// 	if err != nil {
// 		t.Error(err.Error())
// 		return n, repo.DatasetRef{}
// 	}

// 	ref, _, err := SaveDataset(n, tc.Name, tc.Input, tc.BodyFile(), nil, false, true)
// 	if err != nil {
// 		t.Error(err.Error())
// 	}

// 	return n, ref
// }

func TestReadDataset(t *testing.T) {
	// n, ref := createDataset(t, rmf)

	// if err := ReadDataset(n.Repo, &ref); err != nil {
	// 	t.Error(err.Error())
	// 	return
	// }

	// if ref.Dataset == nil {
	// 	t.Error("expected dataset to not equal nil")
	// 	return
	// }
}

// func testRenameDataset(t *testing.T, rmf RepoMakerFunc) {
// 	node, ref := createDataset(t, rmf)

// 	b := &repo.DatasetRef{
// 		Name:     "cities2",
// 		Peername: "me",
// 	}

// 	if err := RenameDataset(node, &ref, b); err != nil {
// 		t.Error(err.Error())
// 		return
// 	}

// 	if err := ReadDataset(node.Repo, b); err != nil {
// 		t.Error(err.Error())
// 		return
// 	}

// 	if b.Dataset == nil {
// 		t.Error("expected dataset to not equal nil")
// 		return
// 	}
// }

func TestDatasetPinning(t *testing.T) {
	r := newTestRepo(t)
	ref := addCitiesDataset(t, r)

	if err := PinDataset(r, ref); err != nil {
		if err == repo.ErrNotPinner {
			t.Log("repo store doesn't support pinning")
		} else {
			t.Error(err.Error())
			return
		}
	}

	tc, err := dstest.NewTestCaseFromDir(testdataPath("counter"))
	if err != nil {
		t.Error(err.Error())
		return
	}

	ref2, err := CreateDataset(r, tc.Name, tc.Input, tc.BodyFile(), false)
	if err != nil {
		t.Error(err.Error())
		return
	}

	if err := PinDataset(r, ref2); err != nil && err != repo.ErrNotPinner {
		t.Error(err.Error())
		return
	}

	if err := UnpinDataset(r, ref); err != nil && err != repo.ErrNotPinner {
		t.Error(err.Error())
		return
	}

	if err := UnpinDataset(r, ref2); err != nil && err != repo.ErrNotPinner {
		t.Error(err.Error())
		return
	}
}

// func testDeleteDataset(t *testing.T, rmf RepoMakerFunc) {
// 	node, ref := createDataset(t, rmf)

// 	if err := DeleteDataset(node, &ref); err != nil {
// 		t.Error(err.Error())
// 		return
// 	}
// }

// func testEventsLog(t *testing.T, rmf RepoMakerFunc) {
// 	node, ref := createDataset(t, rmf)
// 	pinner := true

// 	b := &repo.DatasetRef{
// 		Name:      "cities2",
// 		ProfileID: ref.ProfileID,
// 	}

// 	if err := RenameDataset(node, &ref, b); err != nil {
// 		t.Error(err.Error())
// 		return
// 	}

// 	if err := PinDataset(node.Repo, *b); err != nil {
// 		if err == repo.ErrNotPinner {
// 			pinner = false
// 		} else {
// 			t.Error(err.Error())
// 			return
// 		}
// 	}

// 	// TODO - calling unpin followed by delete will trigger two unpin events,
// 	// which based on our current architecture can and will probably cause problems
// 	// we should either hardern every unpin implementation to not error on multiple
// 	// calls to unpin the same hash, or include checks in the delete method
// 	// and only call unpin if the hash is in fact pinned
// 	// if err := act.UnpinDataset(b); err != nil && err != repo.ErrNotPinner {
// 	// 	t.Error(err.Error())
// 	// 	return
// 	// }

// 	if err := DeleteDataset(node, b); err != nil {
// 		t.Error(err.Error())
// 		return
// 	}

// 	events, err := node.Repo.Events(10, 0)
// 	if err != nil {
// 		t.Error(err.Error())
// 		return
// 	}

// 	ets := []repo.EventType{repo.ETDsDeleted, repo.ETDsUnpinned, repo.ETDsPinned, repo.ETDsRenamed, repo.ETDsPinned, repo.ETDsCreated}

// 	if !pinner {
// 		ets = []repo.EventType{repo.ETDsDeleted, repo.ETDsRenamed, repo.ETDsCreated}
// 	}

// 	if len(events) != len(ets) {
// 		t.Errorf("event log length mismatch. expected: %d, got: %d", len(ets), len(events))
// 		t.Log("event log:")
// 		for i, e := range events {
// 			t.Logf("\t%d: %s", i, e.Type)
// 		}
// 		return
// 	}

// 	for i, et := range ets {
// 		if events[i].Type != et {
// 			t.Errorf("case %d eventType mismatch. expected: %s, got: %s", i, et, events[i].Type)
// 		}
// 	}
// }
