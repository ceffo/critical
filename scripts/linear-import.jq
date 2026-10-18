# Builds the import plan from the beads export. Input: the beads as one array (jq -s); $m: scripts/linear-mapping.json.
# Output: {issues: [...], relations: [{from, to}]}; "from" blocks "to". Pure and deterministic: the shell
# (scripts/linear-import.sh) only executes it. Scope: every epic (closed ones included) and every item that is
# not closed; tombstones and the other closed history are not imported.

# Linear takes millisecond timestamps.
def ms: if . == null then null else sub("\\.(?<f>[0-9]+)Z$"; "." + ((.f + "00")[0:3]) + "Z") end;
def day: .[0:10];
def tick: "`" + . + "`";

. as $all
| ($all | map({key: .id, value: .}) | from_entries) as $byid
| ($all | map(select(.status != "tombstone" and (.status != "closed" or .issue_type == "epic")))) as $S
| ($S | map(.id)) as $ids
| ($S | map(select(.id as $i | $ids | index($i)))) as $imported
| def stat($b; $key): ($m.status[$b.status] // error("no mapping for status " + $b.status))[$key];
  def desc($b):
    ($b.description // "") as $d
    | if ($b.acceptance_criteria // "") != "" and (($d | contains($b.acceptance_criteria)) | not)
      then $d + "\n\n## Acceptance criteria\n" + $b.acceptance_criteria else $d end;
  def whynot($id): ($byid[$id].status // "unknown") as $s | ($id | tick) + " (" + $s + ", not imported)";
  def deps($b; $t): [($b.dependencies // [])[] | select(.issue_id == $b.id and .type == $t) | .depends_on_id];
  {
    issues: [
      $imported | sort_by([(if .issue_type == "epic" then 0 else 1 end), .id])[] | . as $b
      | (deps($b; "parent-child")[0]) as $parent
      | (deps($b; "blocks")) as $blockedby
      | ([$all[] | select(.status != "tombstone") | select(.id as $i | $ids | index($i) | not)
          | select(any((.dependencies // [])[]; .depends_on_id == $b.id and .type == "blocks")) | .id]) as $blocksother
      | (if $b.status == "closed" then "Done" else stat($b; "state") end) as $state
      | ([ ($b.notes // "") | select(. != "") | {token: "imported from beads [notes]",
            body: ("**Notes** (from beads)\n\n" + . + "\n\n_imported from beads [notes]_"), createdAt: ($b.created_at | ms)} ]) as $notes
      | ([ ($parent | select(. != null) | select(. as $x | $ids | index($x) | not) | "Child of " + whynot(.)),
           ($blockedby[] | select(. as $x | $ids | index($x) | not) | "Blocked by " + whynot(.)),
           ($blocksother[] | "Blocks " + whynot(.)) ]) as $unimported
      | {
          id: $b.id,
          epic: ($b.issue_type == "epic"),
          title: ($b.title + " [" + $b.id + "]"),
          description: desc($b),
          state: $state,
          priority: ($m.priority["P" + ($b.priority | tostring)].linear // error("no priority " + ($b.priority | tostring))),
          labels: ((($b.labels // []) + [($m.type[$b.issue_type] // null), (if $b.status == "deferred" then stat($b; "label") else null end)])
                   | map(select(. != null)) | unique),
          createdAt: ($b.created_at | ms),
          completedAt: (if $state == "Done" then ($b.closed_at | ms) else null end),
          parent: (if $parent != null and ($ids | index($parent)) then $parent else null end),
          comments: (
            [($b.comments // [])[] | {token: ("imported from beads [comment " + (.id | tostring) + "]"),
               body: ("**" + .author + "**, " + (.created_at | day) + "\n\n" + .text + "\n\n_imported from beads [comment " + (.id | tostring) + "]_"),
               createdAt: (.created_at | ms)}]
            + $notes
            + (if ($unimported | length) > 0 then [{token: "imported from beads [relations]",
                body: ("Relations to items not imported into Linear (closed history, kept in the beads archive):\n\n" + ($unimported | map("- " + .) | join("\n")) + "\n\n_imported from beads [relations]_"),
                createdAt: null}] else [] end))
        }
    ],
    relations: [ $imported[] | . as $b | deps($b; "blocks")[] | select(. as $x | $ids | index($x)) | {from: ., to: $b.id} ] | unique
  }
