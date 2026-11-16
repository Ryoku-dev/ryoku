// RestBucket gives plan.js the two R2 binding calls gatherFacts makes, list
// and get, over the Cloudflare REST API. It has no delete or put: a plan built
// on it cannot change the bucket.
export class RestBucket {
  constructor({ accountId, bucket, token, fetch = globalThis.fetch, api = "https://api.cloudflare.com/client/v4" }) {
    if (!accountId) throw new Error("an account ID is required");
    if (!bucket) throw new Error("a bucket name is required");
    if (!token) throw new Error("an API token is required");
    this.base = `${api}/accounts/${encodeURIComponent(accountId)}/r2/buckets/${encodeURIComponent(bucket)}/objects`;
    this.token = token;
    this.fetch = fetch;
  }

  async request(url) {
    const response = await this.fetch(url, { headers: { Authorization: `Bearer ${this.token}` } });
    if (response.status === 404) return null;
    if (!response.ok) {
      throw new Error(`R2 REST ${response.status} for ${new URL(url).pathname}`);
    }
    return response;
  }

  async list({ prefix = "", delimiter, cursor } = {}) {
    const url = new URL(this.base);
    url.searchParams.set("per_page", "1000");
    if (prefix) url.searchParams.set("prefix", prefix);
    if (delimiter) url.searchParams.set("delimiter", delimiter);
    if (cursor) url.searchParams.set("cursor", cursor);
    const response = await this.request(url.toString());
    if (!response) throw new Error(`R2 REST 404 listing ${prefix || "/"}`);
    const body = await response.json();
    if (!body.success) throw new Error(`R2 REST listing ${prefix || "/"} failed`);
    const info = body.result_info ?? {};
    const next = info.is_truncated && info.cursor ? info.cursor : undefined;
    return {
      objects: (body.result ?? []).map((object) => ({
        key: object.key,
        size: object.size ?? 0,
        uploaded: object.last_modified,
      })),
      delimitedPrefixes: info.delimited ?? [],
      truncated: Boolean(next),
      cursor: next,
    };
  }

  async get(key) {
    const response = await this.request(`${this.base}/${encodeURIComponent(key)}`);
    if (!response) return null;
    const text = await response.text();
    return { text: async () => text };
  }
}
