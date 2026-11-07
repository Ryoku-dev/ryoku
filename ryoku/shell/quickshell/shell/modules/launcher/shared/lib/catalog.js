function safeQmlPath(value) {
  return typeof value === "string"
    && /^variants\/[a-z0-9-]+\/[A-Za-z][A-Za-z0-9]*\.qml$/.test(value)
    && value.indexOf("..") < 0;
}

function normalize(raw) {
  if (!raw || raw.version !== 1 || !Array.isArray(raw.variants))
    throw new Error("launcher catalog must be version 1");

  var ids = {};
  var variants = raw.variants.map(function (source) {
    var id = String(source.id || "");
    if (!/^[a-z0-9-]+$/.test(id) || ids[id])
      throw new Error("launcher catalog has an invalid or duplicate id: " + id);
    if (!safeQmlPath(source.entrypoint) || !safeQmlPath(source.preview))
      throw new Error("launcher catalog has an unsafe QML path for: " + id);
    ids[id] = true;
    return {
      id: id,
      name: String(source.name || id),
      description: String(source.description || ""),
      entrypoint: source.entrypoint,
      preview: source.preview,
      capabilities: Array.isArray(source.capabilities)
        ? source.capabilities.map(String) : []
    };
  });

  var defaultId = String(raw.default || "");
  var fallbackId = String(raw.fallback || "");
  if (!ids[defaultId] || !ids[fallbackId])
    throw new Error("launcher catalog default and fallback must name entries");
  return { defaultId: defaultId, fallbackId: fallbackId, variants: variants };
}

function find(catalog, id) {
  for (var i = 0; i < catalog.variants.length; i++)
    if (catalog.variants[i].id === id) return catalog.variants[i];
  return null;
}

function defaultEntry(catalog) { return find(catalog, catalog.defaultId); }
function fallbackEntry(catalog) { return find(catalog, catalog.fallbackId); }
function entry(catalog, requestedId) {
  return find(catalog, String(requestedId || "")) || defaultEntry(catalog);
}

function findContribution(contributions, id) {
  var requested = String(id || "");
  var rows = Array.isArray(contributions) ? contributions : [];
  for (var i = 0; i < rows.length; i++)
    if (String(rows[i] && rows[i].id || "") === requested) return rows[i];
  return null;
}

function contributionEntry(contribution) {
  return {
    id: String(contribution.id || ""),
    name: String(contribution.name || contribution.id || ""),
    description: String(contribution.description || ""),
    contribution: true,
    itemId: String(contribution.itemId || ""),
    url: String(contribution.entryUrl || ""),
    previewUrl: String(contribution.previewUrl || ""),
    capabilities: []
  };
}

function resolve(catalog, requestedId, contributions) {
  var requested = String(requestedId || "");
  var catalogEntry = find(catalog, requested);
  if (catalogEntry && catalogEntry.store !== true)
    return { entry: catalogEntry, pending: false };

  var contribution = findContribution(contributions, requested);
  if (contribution) {
    if (contribution.prepared !== true || String(contribution.entryUrl || "") === "")
      return { entry: defaultEntry(catalog), pending: true };
    return { entry: contributionEntry(contribution), pending: false };
  }

  return { entry: catalogEntry || defaultEntry(catalog), pending: false };
}

// A Store launcher runs from its receipt-owned view: launcher-views/<id>/<digest>
// under the Store's state root, holding Main.qml and, when the product ships
// one, Preview.qml.
var STORE_VIEW = /^launcher-views\/([a-z0-9]+(?:-[a-z0-9]+)*)\/[a-f0-9]{64}$/;

// storeVariants turns the Store's launchers.json index rows into catalog
// entries with absolute file URLs. A malformed row, one whose view belongs to
// another id, a repeat, or one that would shadow a shipped launcher is dropped:
// the launcher the shell ships always answers to its own id.
function storeVariants(catalog, rows, stateRoot) {
  var taken = {};
  catalog.variants.forEach(function (variant) { taken[variant.id] = true; });
  var variants = [];
  (Array.isArray(rows) ? rows : []).forEach(function (row) {
    var id = String(row && row.id || "");
    var version = String(row && row.version || "");
    var match = STORE_VIEW.exec(String(row && row.view || ""));
    if (!match || match[1] !== id || version === "" || taken[id])
      return;
    taken[id] = true;
    var root = "file://" + stateRoot + "/" + row.view + "/";
    variants.push({
      id: id,
      name: id,
      description: "",
      version: version,
      store: true,
      url: root + "Main.qml",
      previewUrl: row.preview === "Preview.qml" ? root + "Preview.qml" : "",
      capabilities: []
    });
  });
  return variants;
}

function withStore(catalog, rows, stateRoot) {
  return {
    defaultId: catalog.defaultId,
    fallbackId: catalog.fallbackId,
    variants: catalog.variants.concat(storeVariants(catalog, rows, stateRoot))
  };
}

if (typeof module !== "undefined" && module.exports)
  module.exports = { safeQmlPath, normalize, find, entry, defaultEntry, fallbackEntry, findContribution, contributionEntry, resolve, storeVariants, withStore };
