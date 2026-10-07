export interface ManifestItem {
  label: string;
  path: string;
  kind: string;
  owner: string;
  desc: string;
  exists: boolean;
}

export interface ManifestInfo {
  skill: ManifestItem;
  prowl: ManifestItem;
  vault: ManifestItem[];
  snippet: string;
}

export interface AboutInfo {
  version: string;
  port: number;
  vault: string;
  hermes: {
    installed: boolean;
    version: string;
    configured: boolean;
    provider: string;
    model: string;
  };
  prowl: { installed: boolean };
}

export type AboutManifest = ManifestInfo;
