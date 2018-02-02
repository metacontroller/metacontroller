// Just run this file with the jsonnet CLI and check the summary at the bottom.

local vitess = import "../hooks/vitess.libsonnet";

local test = function(name, got, want)
  local passed = got == want;
  {
    name: name,
    passed: passed,
    [if !passed then "got"]: got,
    [if !passed then "want"]: want,
  };

local tests = [
  local table = [
    {flags: {}, want: ""},
    {
      flags: {
        port: 15000,
        mysqlctl_socket: "$VTDATAROOT/mysqlctl.sock",
        enable_semi_sync: true,
        "db-config-app-uname": "vt_app",
      },
      want: "-db-config-app-uname=\"vt_app\" -enable_semi_sync=\"true\" -mysqlctl_socket=\"$VTDATAROOT/mysqlctl.sock\" -port=\"15000\""
    },
  ];
  test("formatFlags",
    [vitess.formatFlags(t.flags) for t in table]
    ,
    want = [t.want for t in table]
  ),

  local table = [
    {cell: "zone1", keyspace: "main", shard: "-80", type: "replica", index: 0, uid: 214747900},
    {cell: "zone1", keyspace: "main", shard: "-80", type: "replica", index: 15, uid: 214747915},
  ];
  test("tabletUid",
    [vitess.tabletUid(t) for t in table]
    ,
    want = [t.uid for t in table]
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
