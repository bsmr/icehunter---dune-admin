package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// fileHash returns a content hash of path, or "" if it does not exist.
func fileHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return string(sum[:])
}

// Once the DB-backed stores are live, config.yaml is import-seed-only: no
// runtime save (global settings, add server, per-server edit, feature flags)
// may ever rewrite it.
func TestConfigYAML_WrittenOnceThenNeverAgain(t *testing.T) {
	t.Setenv("DUNE_ADMIN_CONFIG_DIR", t.TempDir())
	db := openSharedScopeDB(t)
	useTestServerStores(t, db)

	origCfg := loadedConfig
	origReg := globalRegistry
	loadedConfig = appConfig{ListenAddr: ":8080"}
	globalRegistry = newServerRegistry(nil)
	t.Cleanup(func() { loadedConfig = origCfg; globalRegistry = origReg })

	// First-boot import seed.
	if err := writeConfigFile(loadedConfig); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	hydrateConfigFromStore()
	before := fileHash(t, configPath())
	if before == "" {
		t.Fatal("config.yaml missing after seed")
	}

	// 1) Global-settings save (scope=global).
	body, _ := json.Marshal(appConfig{ListenAddr: ":9999", DiscordBotToken: "tok"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config?scope=global", bytes.NewReader(body))
	handleSaveConfig(httptest.NewRecorder(), req)

	// 2) Add a server.
	addBody, _ := json.Marshal(map[string]any{"name": "Two", "control": "local"})
	handleAddServer(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/servers", bytes.NewReader(addBody)))

	// 3) Feature-flag save (battlepass) routes through persistGlobalSettings.
	loadedConfig.BattlepassEnabled = boolPtr(true)
	if err := persistGlobalSettings(loadedConfig); err != nil {
		t.Fatalf("persistGlobalSettings: %v", err)
	}

	if after := fileHash(t, configPath()); after != before {
		t.Error("config.yaml was rewritten after a runtime save; it must be import-seed-only in DB mode")
	}

	// The global save must have landed in the DB instead.
	if cfg, ok, _ := globalSettingsStore.loadSettings(); !ok || cfg.ListenAddr != ":9999" {
		t.Errorf("global save did not persist to the DB: ok=%v listen=%q", ok, cfg.ListenAddr)
	}
}

func TestImportConfigYAML_MultiServerSeedsServers(t *testing.T) {
	db := openSharedScopeDB(t)
	useTestServerStores(t, db)

	seed := appConfig{
		ListenAddr: ":9090", // global setting → app_settings
		Servers: []ServerConfig{
			{LegacyID: "s1", Name: "One", Control: "local"},
			{LegacyID: "s2", Name: "Two", Control: "amp"},
		},
	}
	if err := importConfigYAMLIntoStore(seed); err != nil {
		t.Fatalf("import: %v", err)
	}

	// Servers persisted with numeric ids in order.
	list, _ := globalServersStore.listServers()
	if len(list) != 2 {
		t.Fatalf("servers = %d, want 2", len(list))
	}
	if list[0].Name != "One" || list[1].Name != "Two" {
		t.Errorf("server order wrong: %+v", list)
	}

	// Global settings persisted.
	if cfg, ok, _ := globalSettingsStore.loadSettings(); !ok || cfg.ListenAddr != ":9090" {
		t.Errorf("settings not persisted: ok=%v listen=%q", ok, cfg.ListenAddr)
	}

	// The new server's numeric scope is usable for per-feature data.
	newScope := serverScope(list[0].ID)
	if err := newWelcomeStore(db, list[0].ID).insertGranted("FLS1", "v1", 1, "Paul"); err != nil {
		t.Fatalf("insert grant under new scope: %v", err)
	}
	if ex, _ := newWelcomeStore(db, list[0].ID).grantExists("FLS1", "v1", 1); !ex {
		t.Errorf("welcome grant not visible under new scope %q", newScope)
	}

	// Active server + marker set (active marker is the first server's string scope).
	if v, _ := metaGet(db, activeServerMetaKey); v != newScope {
		t.Errorf("active = %q, want %q", v, newScope)
	}
	if v, _ := metaGet(db, configImportMarker); v == "" {
		t.Error("import marker not written")
	}
}

func TestImportConfigYAML_LegacyFlatSingleServer(t *testing.T) {
	db := openSharedScopeDB(t)
	useTestServerStores(t, db)

	// No Servers[] — a flat single-server config. Stub the flag globals that
	// flatConfigHasConnection inspects.
	origPass := dbPass
	dbPass = "secret"
	t.Cleanup(func() { dbPass = origPass })

	if err := importConfigYAMLIntoStore(appConfig{DBPass: "secret", Control: "local"}); err != nil {
		t.Fatalf("import: %v", err)
	}

	list, _ := globalServersStore.listServers()
	if len(list) != 1 {
		t.Fatalf("servers = %d, want 1 (legacy flat → one server)", len(list))
	}
	// The single server's numeric scope is usable for per-feature data.
	if err := newWelcomeStore(db, list[0].ID).insertGranted("FLS9", "v1", 7, "Chani"); err != nil {
		t.Fatalf("insert grant under flat server scope: %v", err)
	}
	if ex, _ := newWelcomeStore(db, list[0].ID).grantExists("FLS9", "v1", 7); !ex {
		t.Errorf("welcome grant not visible under flat server scope %d", list[0].ID)
	}
}

// A fresh install (no config.yaml) must NOT import a phantom server from the
// env/default flag-globals, and must NOT write the import marker (so a
// config.yaml dropped in later still imports on its first boot).
func TestHydrateConfigFromStore_NoConfigYAMLImportsNothing(t *testing.T) {
	t.Setenv("DUNE_ADMIN_CONFIG_DIR", t.TempDir()) // empty dir → no config.yaml
	db := openMemUnifiedStore(t)
	useTestServerStores(t, db)

	origCfg := loadedConfig
	origPass := dbPass
	// Simulate env/default flat connection that would otherwise seed a phantom.
	dbPass = "from-env"
	loadedConfig = appConfig{DBPass: "from-env", DBHost: "127.0.0.1", Control: "local"}
	t.Cleanup(func() { loadedConfig = origCfg; dbPass = origPass })

	hydrateConfigFromStore()

	if has, _ := globalServersStore.hasAnyServer(); has {
		t.Error("fresh install imported a phantom server; want none")
	}
	if v, _ := metaGet(db, configImportMarker); v != "" {
		t.Error("import marker written on fresh install; want unset so a later config.yaml still imports")
	}
	if !needsSetup() {
		t.Error("needsSetup() should be true on a fresh install with no servers")
	}
}

func TestHydrateConfigFromStore_ImportsOnceThenIdempotent(t *testing.T) {
	t.Setenv("DUNE_ADMIN_CONFIG_DIR", t.TempDir())
	db := openSharedScopeDB(t)
	useTestServerStores(t, db)

	origCfg := loadedConfig
	loadedConfig = appConfig{
		ListenAddr: ":7000",
		Servers:    []ServerConfig{{LegacyID: "s1", Name: "One", Control: "local"}},
	}
	t.Cleanup(func() { loadedConfig = origCfg })

	// hydrate imports only when a real config.yaml exists.
	if err := writeConfigFile(loadedConfig); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	hydrateConfigFromStore()

	// Marker written; loadedConfig.Servers now carry DB numeric ids.
	if v, _ := metaGet(db, configImportMarker); v == "" {
		t.Fatal("marker not written after first hydrate")
	}
	if len(loadedConfig.Servers) != 1 || loadedConfig.Servers[0].ID == 0 {
		t.Fatalf("loadedConfig.Servers not hydrated with numeric id: %+v", loadedConfig.Servers)
	}
	firstID := loadedConfig.Servers[0].ID
	if loadedConfig.DefaultServer != serverScope(firstID) {
		t.Errorf("DefaultServer = %q, want %q", loadedConfig.DefaultServer, serverScope(firstID))
	}

	// Second hydrate must NOT re-import (marker guards it) → still exactly one server.
	hydrateConfigFromStore()
	list, _ := globalServersStore.listServers()
	if len(list) != 1 {
		t.Errorf("servers = %d after second hydrate, want 1 (no re-import)", len(list))
	}
}

// ── #278: amp_container_runtime backfill for flat (single-server) configs ────
//
// An operator on the legacy flat config.yaml shape (no Servers[]) who adds
// `amp_container_runtime: docker` *after* their first boot (marker already
// written) previously had it silently ignored forever: the config.yaml -> DB
// import only runs once. The default server's DB row kept an empty
// amp_container_runtime, so runtimeCLI() defaulted to "podman" and every AMP
// API call failed with "podman: command not found" on a docker install.

func TestBackfillAmpContainerRuntime_FillsEmptyFromFlatConfig(t *testing.T) {
	t.Parallel()
	servers := []ServerConfig{{ID: 1, AmpContainerRuntime: ""}, {ID: 2, AmpContainerRuntime: ""}}
	backfillAmpContainerRuntime("docker", servers)
	if servers[0].AmpContainerRuntime != "docker" {
		t.Errorf("servers[0].AmpContainerRuntime = %q, want %q", servers[0].AmpContainerRuntime, "docker")
	}
	if servers[1].AmpContainerRuntime != "" {
		t.Errorf("servers[1] must be untouched, got %q", servers[1].AmpContainerRuntime)
	}
}

func TestBackfillAmpContainerRuntime_NoopWhenFlatRuntimeEmpty(t *testing.T) {
	t.Parallel()
	// Multi-server configs have no top-level amp_container_runtime — must be a no-op.
	servers := []ServerConfig{{ID: 1, AmpContainerRuntime: ""}}
	backfillAmpContainerRuntime("", servers)
	if servers[0].AmpContainerRuntime != "" {
		t.Errorf("expected no-op with empty flatRuntime, got %q", servers[0].AmpContainerRuntime)
	}
}

func TestBackfillAmpContainerRuntime_NeverOverwritesAnExistingValue(t *testing.T) {
	t.Parallel()
	// The operator already set (or the first import already seeded) a runtime —
	// must never be clobbered, even if config.yaml now says something else.
	servers := []ServerConfig{{ID: 1, AmpContainerRuntime: "podman"}}
	backfillAmpContainerRuntime("docker", servers)
	if servers[0].AmpContainerRuntime != "podman" {
		t.Errorf("existing runtime overwritten: got %q, want %q", servers[0].AmpContainerRuntime, "podman")
	}
}

func TestBackfillAmpContainerRuntime_NoopWhenNoServers(t *testing.T) {
	t.Parallel()
	backfillAmpContainerRuntime("docker", nil) // must not panic
}

// TestHydrateConfigFromStore_BackfillsAmpContainerRuntimeOnLaterBoot is the
// end-to-end regression test for #278: simulate a server that was already
// imported (marker set, runtime empty), then a later boot with
// `amp_container_runtime: docker` freshly present in config.yaml — the
// existing DB row must be corrected without a second import.
func TestHydrateConfigFromStore_BackfillsAmpContainerRuntimeOnLaterBoot(t *testing.T) {
	t.Setenv("DUNE_ADMIN_CONFIG_DIR", t.TempDir())
	db := openSharedScopeDB(t)
	useTestServerStores(t, db)

	origCfg := loadedConfig
	origControlPlane := controlPlane
	t.Cleanup(func() { loadedConfig = origCfg; controlPlane = origControlPlane })

	// flatConfigHasConnection() checks the flag-globals (not the appConfig
	// struct), so a real single-server import needs one set — mirrors
	// TestImportConfigYAML_LegacyFlatSingleServer.
	controlPlane = "amp"

	// First boot: flat config, no amp_container_runtime configured yet.
	loadedConfig = appConfig{ListenAddr: ":8080", Control: "amp"}
	if err := writeConfigFile(loadedConfig); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	hydrateConfigFromStore()

	list, err := globalServersStore.listServers()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 imported server, got %d (err=%v)", len(list), err)
	}
	if list[0].AmpContainerRuntime != "" {
		t.Fatalf("precondition: expected empty runtime after first import, got %q", list[0].AmpContainerRuntime)
	}

	// Later boot: operator added amp_container_runtime to config.yaml. The
	// import marker is already set, so importConfigYAMLIntoStore must NOT
	// run again — only the backfill should correct the stored runtime.
	loadedConfig = appConfig{ListenAddr: ":8080", Control: "amp", AmpContainerRuntime: "docker"}
	hydrateConfigFromStore()

	list, err = globalServersStore.listServers()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected still exactly 1 server (no re-import), got %d (err=%v)", len(list), err)
	}
	if list[0].AmpContainerRuntime != "docker" {
		t.Errorf("DB server runtime = %q, want %q (backfilled)", list[0].AmpContainerRuntime, "docker")
	}
	if loadedConfig.Servers[0].AmpContainerRuntime != "docker" {
		t.Errorf("loadedConfig.Servers[0].AmpContainerRuntime = %q, want %q (same-boot visibility)",
			loadedConfig.Servers[0].AmpContainerRuntime, "docker")
	}
}
