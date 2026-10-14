export interface FileVersion {
  hash: string;
  shortHash: string;
  author: string;
  email: string;
  subject: string;
  date: string;
  parent: string;
  merge: boolean;
  beforePath: string;
  afterPath: string;
  change: string;
}

export interface FileHistory {
  root: string;
  path: string;
  head: string;
  versions: FileVersion[];
}

export interface Snapshot {
  content: string;
  exists: boolean;
  binary: boolean;
  truncated: boolean;
}

export interface ReviewLink {
  provider: "github" | "gitlab";
  number: number;
  url: string;
}

export interface Reviews {
  links: ReviewLink[];
}

export type Request =
  | { operation: "history"; filePath: string }
  | { operation: "snapshot"; root: string; revision: string; path: string }
  | { operation: "reviews"; root: string; revision: string };

export interface Response<T> {
  protocolVersion: number;
  result?: T;
  error?: { code: string; message: string };
}

export interface HistoryEntry {
  root: string;
  version: FileVersion;
}

export interface HistoryState {
  context: number;
  target?: string;
  history?: FileHistory;
  selectedHash?: string;
  pinned: boolean;
  loading: boolean;
  error?: string;
  empty?: boolean;
  reviews?: Record<string, ReviewLink[]>;
  reviewLoadingHash?: string;
  reviewError?: string;
}

