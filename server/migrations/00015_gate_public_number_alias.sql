-- +goose Up
ALTER TABLE gates
  ADD COLUMN public_id integer,
  ADD COLUMN alias text;

WITH ranked AS (
  SELECT gate.workspace_id,
         gate.id,
         row_number() OVER (
           PARTITION BY gate.workspace_id
           ORDER BY phase.position,gate.id
         )::integer AS public_id
  FROM gates gate
  JOIN phases phase
    ON phase.workspace_id=gate.workspace_id
   AND phase.id=gate.from_phase_id
)
UPDATE gates gate
SET public_id=ranked.public_id
FROM ranked
WHERE gate.workspace_id=ranked.workspace_id
  AND gate.id=ranked.id;

ALTER TABLE gates
  ALTER COLUMN public_id SET NOT NULL,
  ADD CONSTRAINT gates_public_id_positive CHECK (public_id > 0),
  ADD CONSTRAINT gates_alias_nonempty CHECK (alias IS NULL OR btrim(alias) <> ''),
  ADD CONSTRAINT gates_alias_canonical CHECK (alias IS NULL OR alias = lower(btrim(alias)));

CREATE UNIQUE INDEX gates_workspace_public_id_uq
  ON gates(workspace_id,public_id);
CREATE UNIQUE INDEX gates_workspace_alias_uq
  ON gates(workspace_id,lower(alias))
  WHERE alias IS NOT NULL;

ALTER TABLE workspace_counters
  ADD COLUMN next_gate_public_id integer NOT NULL DEFAULT 1
  CHECK (next_gate_public_id > 0);

UPDATE workspace_counters counter
SET next_gate_public_id=COALESCE((
  SELECT max(gate.public_id)+1
  FROM gates gate
  WHERE gate.workspace_id=counter.workspace_id
),1);

-- +goose Down
ALTER TABLE workspace_counters DROP COLUMN next_gate_public_id;
DROP INDEX gates_workspace_alias_uq;
DROP INDEX gates_workspace_public_id_uq;
ALTER TABLE gates
  DROP CONSTRAINT gates_alias_canonical,
  DROP CONSTRAINT gates_alias_nonempty,
  DROP CONSTRAINT gates_public_id_positive,
  DROP COLUMN alias,
  DROP COLUMN public_id;
