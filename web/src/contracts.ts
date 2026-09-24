export type AngleUnit = 'deg' | 'rad';

export type Capabilities = {
  semanticsVersion: string;
  operators: string[];
  functions: Record<string, number[]>;
  angleUnits: AngleUnit[];
  defaultAngleUnit: AngleUnit;
  limits: {
    expressionLength: number;
    tokens: number;
    nesting: number;
  };
  features: {
    factorial: boolean;
    percentage: boolean;
    remainder: boolean;
    statistics: boolean;
    achievements: boolean;
    themes: boolean;
    minimalPresentation: boolean;
    localization: boolean;
    reductionPlayback: boolean;
    rooms: boolean;
    roomReactions: boolean;
    roomPublicationControl: boolean;
  };
};

export type SourceSpan = {
  start: number;
  end: number;
};

export type MathError = {
  code: string;
  stage: 'parse' | 'evaluate';
  params: Record<string, unknown>;
  span: SourceSpan | null;
};

export type Outcome =
  | { kind: 'success'; value: string }
  | { kind: 'error'; error: MathError };

export type CalculationContext = {
  angleUnit: AngleUnit;
  semanticsVersion: string;
};

export type CalculationFacts = {
  operators: Record<string, number>;
  functions: Record<string, number>;
  operationCount: number;
  depth: number;
};

export type RoomContext = {
  code: string;
  publish: boolean;
};

export type CalculationRequest = {
  requestId: string;
  expression: string;
  angleUnit: AngleUnit;
  room?: RoomContext;
};

export type CalculationRecord = {
  id: string;
  requestId: string;
  expression: string;
  context: CalculationContext;
  outcome: Outcome;
  facts?: CalculationFacts;
  createdAt: string;
};

export type CalculationResponse = {
  calculation: CalculationRecord;
  publication: { status: 'private' | 'published' | 'unavailable' };
  achievements?: Achievement[];
  funEvents?: FunEvent[];
};

export type HistoryPage = {
  items: CalculationRecord[];
  nextCursor: string | null;
};

export type LongestExpression = {
  calculationId: string;
  expression: string;
  length: number;
};

export type PersonalStatistics = {
  totalCalculations: number;
  successes: number;
  mathematicalErrors: number;
  divisionByZeroAttempts: number;
  operators: Record<string, number>;
  functions: Record<string, number>;
  longestExpression: LongestExpression | null;
  maxParsedDepth: number | null;
};

export type Achievement = {
  id: string;
  earnedAt: string;
};

export type DiscoveryText = {
  name: string;
  description: string;
  comment: string;
};

export type DiscoveryDefinition = {
  id: string;
  ru: DiscoveryText;
  en: DiscoveryText;
};

export type FunEvent = {
  id: string;
  ruleId: string;
  kind: 'comment' | 'scene';
  scope: 'personal' | 'room';
  params: Record<string, unknown>;
  createdAt: string;
  expiresAt: string;
};

export type SessionResponse = {
  alias: string;
  identity: string;
  achievements?: Achievement[];
  discoveryCatalog?: DiscoveryDefinition[];
  discoveriesAvailable: boolean;
};

export type ErrorResponse = {
  error: { code: string; params: Record<string, unknown> };
};
