import type { DomainDefinition } from "@flanksource/clicky-ui";

export type DomainSpec = {
  definition: DomainDefinition;
  entities: string[];
  // When true, the OperationCatalog displays all operations regardless of
  // entity tags. Use for an API Explorer-style view.
  allOperations?: boolean;
  operationIdPrefix?: string;
  listOperationId?: string;
  detailOperationId?: string;
};

export const domains: Record<string, DomainSpec> = {
  stacks: {
    definition: {
      key: "stacks",
      title: "Stacks",
      description:
        "Deployable stacks (checkout, billing, …). Lists, filters, and runs entity actions registered with clicky.",
    },
    entities: ["stack"],
  },
  clusters: {
    definition: {
      key: "clusters",
      title: "Clusters",
      description:
        "Cloud clusters backing the demo stacks. Nested under the `catalog` parent in the entity registration.",
    },
    entities: ["cluster"],
    operationIdPrefix: "catalog_",
    listOperationId: "catalog_cluster_list",
    detailOperationId: "catalog_cluster_get",
  },
  "admin-stacks": {
    definition: {
      key: "admin-stacks",
      title: "Admin — Stacks",
      description:
        "Administrative view of the stack entity. Surfaces archived rows and admin-only fields (secret material, reconcile metadata).",
    },
    entities: ["stack"],
    operationIdPrefix: "admin_",
    listOperationId: "admin_stack_list",
    detailOperationId: "admin_stack_get",
  },
  explorer: {
    definition: {
      key: "explorer",
      title: "API Explorer",
      description: "Every operation exposed by the entity demo's OpenAPI spec.",
    },
    entities: [],
    allOperations: true,
  },
};

export const domainOrder: Array<keyof typeof domains> = [
  "stacks",
  "clusters",
  "admin-stacks",
  "explorer",
];
