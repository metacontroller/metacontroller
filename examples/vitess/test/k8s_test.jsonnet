// Just run this file with the jsonnet CLI and check the summary at the bottom.

local k8s = import "../hooks/k8s.libsonnet";

local test = function(name, got, want)
  local passed = got == want;
  {
    name: name,
    passed: passed,
    [if !passed then "got"]: got,
    [if !passed then "want"]: want,
  };

local tests = [
  test("mergeListMap",
    local lhs = [
      // Should preserve original list order.
      {name: "z", value: 2},
      {name: "a", value: "a"},
    ];
    local rhs = [
      {name: "a", extra: 3, value+: "a"},
      {name: "b", extra: 4},
    ];
    k8s.mergeListMap(lhs, rhs)
    ,
    want = [
      {name: "z", value: 2},
      {name: "a", value: "aa", extra: 3},
      {name: "b", extra: 4},
    ]
  ),

  test("makePatch",
    {
      obj: {
        changed: {initial: 1},
        unchanged: {initial: 2},
        list: [1,2,3],
        // A Kubernetes "list map" keyed by the "name" field.
        listMap: [
          {name: "item2", value: 2},
          {name: "item1", value: 1},
        ],
      },
    }
    +
    k8s.makePatch({
      // This should behave as if it were "obj+: { ... }".
      obj: {
        // makePatch() should recursively apply to fields inside "obj",
        // so this should behave as if it were "changed+: { ... }".
        changed: {extra: 4},
        // This should replace the whole list.
        list: [5,6,7],
        // This should behave as if it used k8s.mergeListMap().
        listMap: [
          {name: "item1", value: 10, extra: 3},
        ],
        // A listMap that doesn't exist in the original object.
        newListMap: [
          {name: "item1", value: 11},
        ],
      },
    })
    ,
    want = {
      obj: {
        changed: {initial: 1, extra: 4},
        unchanged: {initial: 2},
        list: [5,6,7],
        listMap: [
          {name: "item2", value: 2},
          {name: "item1", value: 10, extra: 3},
        ],
        newListMap: [
          {name: "item1", value: 11},
        ],
      },
    }
  ),

  local obj = {metadata:{labels:{a:1,b:2}}};
  local table = [
    {labels: {a:1}, match: true},
    {labels: {a:2}, match: false},
    {labels: {b:1}, match: false},
    {labels: {b:2}, match: true},
    {labels: {c:1}, match: false},
    {labels: {a:1,b:2}, match: true},
    {labels: {a:2,b:2}, match: false},
    {labels: {a:1,b:3}, match: false},
  ];
  test("matchLabels",
    [k8s.matchLabels(obj, t.labels) for t in table]
    ,
    want = [t.match for t in table]
  ),
];

{
  all: tests,
  summary: {
    total: std.length(tests),
    passed: std.length(std.filter(function(t) t.passed, tests)),
    failed: self.total - self.passed,
  },
}
