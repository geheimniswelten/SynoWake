package synowake

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestFavoritePersistsWithoutChangingDeviceOrSchedules(t *testing.T) {
	a := testApp(t)
	d := testDevice(t, a)
	if d.Favorite {
		t.Fatal("new device is a favorite without opting in")
	}
	if err := a.update(func(s *Store) error {
		s.Devices[0].LastWake = time.Now().UTC().Truncate(time.Second)
		s.Devices[0].WakeState = "waking"
		s.Schedules = []savedSchedule{{Schedule: Schedule{ID: randomID(), DeviceID: d.ID, Name: "Weckzeit"}, Secret: "unchanged"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	before := testStore(t, a)
	for _, favorite := range []bool{true, false, true} {
		testServe(t, a, testRequest(t, a, "POST", "device-favorite", map[string]any{"id": d.ID, "favorite": favorite}), 200)
		reopened := &App{Root: a.Root, Demo: true}
		after := testStore(t, reopened)
		want := before.Devices[0]
		want.Favorite = favorite
		if !reflect.DeepEqual(after.Devices[0], want) || !reflect.DeepEqual(after.Schedules, before.Schedules) {
			t.Fatal("favorite change altered other device fields or schedules, or was not persisted")
		}
		result := testServe(t, reopened, testRequest(t, reopened, "GET", "state", nil), 200)
		var state struct {
			Devices []Device `json:"devices"`
		}
		if err := json.Unmarshal(result.Data, &state); err != nil || len(state.Devices) != 1 || state.Devices[0].Favorite != favorite {
			t.Fatalf("favorite missing from reopened UI state: %s", result.Data)
		}
	}
	stored := testStore(t, a).Devices[0]
	stored.Name = "Geänderter Name"
	testServe(t, a, testRequest(t, a, "POST", "device-save", stored), 200)
	if !testStore(t, a).Devices[0].Favorite {
		t.Fatal("editing a favorite removed its selection")
	}
}

func TestFavoriteRejectsInvalidOrUnauthenticatedChanges(t *testing.T) {
	a := testApp(t)
	d := testDevice(t, a)
	for _, payload := range []any{
		map[string]any{"id": d.ID},
		map[string]any{"id": d.ID, "favorite": nil},
		map[string]any{"id": d.ID, "favorite": "true"},
		map[string]any{"id": randomID(), "favorite": true},
		map[string]any{"id": d.ID, "favorite": true, "name": "Not part of this action"},
	} {
		testServe(t, a, testRequest(t, a, "POST", "device-favorite", payload), 400)
	}
	r := testRequest(t, a, "POST", "device-favorite", map[string]any{"id": d.ID, "favorite": true})
	r.Header.Del("X-SynoWake-CSRF")
	testServe(t, a, r, http.StatusForbidden)
	if testStore(t, a).Devices[0].Favorite {
		t.Fatal("rejected request changed the favorite")
	}
}

func TestExistingInventoryWithoutFavoritesRemainsAvailable(t *testing.T) {
	a := &App{Root: t.TempDir()}
	legacy := `{"version":1,"active":true,"csrfSecret":"legacy","devices":[{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","name":"Vorhandener PC","ip":"192.168.1.20","mac":"02:11:22:33:44:55","broadcast":"192.168.1.255","port":9}]}`
	if err := os.WriteFile(filepath.Join(a.Root, "state.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	s := testStore(t, a)
	if len(s.Devices) != 1 || s.Devices[0].Name != "Vorhandener PC" || s.Devices[0].Favorite {
		t.Fatal("existing device lost or implicitly selected as favorite")
	}
}
