// Copyright 2015, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package endtoend

import (
	"testing"

	"github.com/youtube/vitess/go/vt/tabletserver/endtoend/framework"
)

var frameworkErrors = `fail failed:
Result mismatch:
'[[1 1] [1 2]]' does not match
'[[2 1] [1 2]]'
RowsAffected mismatch: 2, want 1
Rewritten mismatch:
'[select eid, id from vtocc_a where 1 != 1 union select eid, id from vtocc_b where 1 != 1 select /* fail */ eid, id from vtocc_a union select eid, id from vtocc_b]' does not match
'[select eid id from vtocc_a where 1 != 1 union select eid, id from vtocc_b where 1 != 1 select /* fail */ eid, id from vtocc_a union select eid, id from vtocc_b]'
Plan mismatch: PASS_SELECT, want aa
Hits mismatch on table stats: 0, want 1
Hits mismatch on query info: 0, want 1
Misses mismatch on table stats: 0, want 2
Misses mismatch on query info: 0, want 2
Absent mismatch on table stats: 0, want 3
Absent mismatch on query info: 0, want 3`

func TestTheFramework(t *testing.T) {
	client := framework.NewDefaultClient()

	expectFail := framework.TestCase{
		Name:  "fail",
		Query: "select /* fail */ eid, id from vtocc_a union select eid, id from vtocc_b",
		Result: [][]string{
			[]string{"2", "1"},
			[]string{"1", "2"},
		},
		RowsAffected: 1,
		Rewritten: []string{
			"select eid id from vtocc_a where 1 != 1 union select eid, id from vtocc_b where 1 != 1",
			"select /* fail */ eid, id from vtocc_a union select eid, id from vtocc_b",
		},
		Plan:   "aa",
		Table:  "bb",
		Hits:   1,
		Misses: 2,
		Absent: 3,
	}
	err := expectFail.Test("", client)
	if err == nil || err.Error() != frameworkErrors {
		t.Errorf("Framework result: \n%q\nexpecting\n%q", err.Error(), frameworkErrors)
	}
}

func TestNocacheCases(t *testing.T) {
	client := framework.NewDefaultClient()

	testCases := []framework.Testable{
		&framework.TestCase{
			Name:  "union",
			Query: "select /* union */ eid, id from vtocc_a union select eid, id from vtocc_b",
			Result: [][]string{
				{"1", "1"},
				{"1", "2"},
			},
			Rewritten: []string{
				"select eid, id from vtocc_a where 1 != 1 union select eid, id from vtocc_b where 1 != 1",
				"select /* union */ eid, id from vtocc_a union select eid, id from vtocc_b",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "double union",
			Query: "select /* double union */ eid, id from vtocc_a union select eid, id from vtocc_b union select eid, id from vtocc_d",
			Result: [][]string{
				{"1", "1"},
				{"1", "2"},
			},
			Rewritten: []string{
				"select eid, id from vtocc_a where 1 != 1 union select eid, id from vtocc_b where 1 != 1 union select eid, id from vtocc_d where 1 != 1",
				"select /* double union */ eid, id from vtocc_a union select eid, id from vtocc_b union select eid, id from vtocc_d",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "distinct",
			Query: "select /* distinct */ distinct * from vtocc_a",
			Result: [][]string{
				{"1", "1", "abcd", "efgh"},
				{"1", "2", "bcde", "fghi"},
			},
			Rewritten: []string{
				"select * from vtocc_a where 1 != 1",
				"select /* distinct */ distinct * from vtocc_a limit 10001",
			},
		},
		&framework.TestCase{
			Name:  "group by",
			Query: "select /* group by */ eid, sum(id) from vtocc_a group by eid",
			Result: [][]string{
				{"1", "3"},
			},
			Rewritten: []string{
				"select eid, sum(id) from vtocc_a where 1 != 1",
				"select /* group by */ eid, sum(id) from vtocc_a group by eid limit 10001",
			},
			RowsAffected: 1,
		},
		&framework.TestCase{
			Name:  "having",
			Query: "select /* having */ sum(id) from vtocc_a having sum(id) = 3",
			Result: [][]string{
				{"3"},
			},
			Rewritten: []string{
				"select sum(id) from vtocc_a where 1 != 1",
				"select /* having */ sum(id) from vtocc_a having sum(id) = 3 limit 10001",
			},
			RowsAffected: 1,
		},
		&framework.TestCase{
			Name:  "limit",
			Query: "select /* limit */ eid, id from vtocc_a limit :a",
			BindVars: map[string]interface{}{
				"a": 1,
			},
			Result: [][]string{
				{"1", "1"},
			},
			Rewritten: []string{
				"select eid, id from vtocc_a where 1 != 1",
				"select /* limit */ eid, id from vtocc_a limit 1",
			},
			RowsAffected: 1,
		},
		&framework.TestCase{
			Name:  "multi-table",
			Query: "select /* multi-table */ a.eid, a.id, b.eid, b.id  from vtocc_a as a, vtocc_b as b order by a.eid, a.id, b.eid, b.id",
			Result: [][]string{
				{"1", "1", "1", "1"},
				{"1", "1", "1", "2"},
				{"1", "2", "1", "1"},
				{"1", "2", "1", "2"},
			},
			Rewritten: []string{
				"select a.eid, a.id, b.eid, b.id from vtocc_a as a, vtocc_b as b where 1 != 1",
				"select /* multi-table */ a.eid, a.id, b.eid, b.id from vtocc_a as a, vtocc_b as b order by a.eid asc, a.id asc, b.eid asc, b.id asc limit 10001",
			},
			RowsAffected: 4,
		},
		&framework.TestCase{
			Name:  "join",
			Query: "select /* join */ a.eid, a.id, b.eid, b.id from vtocc_a as a join vtocc_b as b on a.eid = b.eid and a.id = b.id",
			Result: [][]string{
				{"1", "1", "1", "1"},
				{"1", "2", "1", "2"},
			},
			Rewritten: []string{
				"select a.eid, a.id, b.eid, b.id from vtocc_a as a join vtocc_b as b where 1 != 1",
				"select /* join */ a.eid, a.id, b.eid, b.id from vtocc_a as a join vtocc_b as b on a.eid = b.eid and a.id = b.id limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "straight_join",
			Query: "select /* straight_join */ a.eid, a.id, b.eid, b.id from vtocc_a as a straight_join vtocc_b as b on a.eid = b.eid and a.id = b.id",
			Result: [][]string{
				{"1", "1", "1", "1"},
				{"1", "2", "1", "2"},
			},
			Rewritten: []string{
				"select a.eid, a.id, b.eid, b.id from vtocc_a as a straight_join vtocc_b as b where 1 != 1",
				"select /* straight_join */ a.eid, a.id, b.eid, b.id from vtocc_a as a straight_join vtocc_b as b on a.eid = b.eid and a.id = b.id limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "cross join",
			Query: "select /* cross join */ a.eid, a.id, b.eid, b.id from vtocc_a as a cross join vtocc_b as b on a.eid = b.eid and a.id = b.id",
			Result: [][]string{
				{"1", "1", "1", "1"},
				{"1", "2", "1", "2"},
			},
			Rewritten: []string{
				"select a.eid, a.id, b.eid, b.id from vtocc_a as a cross join vtocc_b as b where 1 != 1",
				"select /* cross join */ a.eid, a.id, b.eid, b.id from vtocc_a as a cross join vtocc_b as b on a.eid = b.eid and a.id = b.id limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "natural join",
			Query: "select /* natural join */ a.eid, a.id, b.eid, b.id from vtocc_a as a natural join vtocc_b as b",
			Result: [][]string{
				{"1", "1", "1", "1"},
				{"1", "2", "1", "2"},
			},
			Rewritten: []string{
				"select a.eid, a.id, b.eid, b.id from vtocc_a as a natural join vtocc_b as b where 1 != 1",
				"select /* natural join */ a.eid, a.id, b.eid, b.id from vtocc_a as a natural join vtocc_b as b limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "left join",
			Query: "select /* left join */ a.eid, a.id, b.eid, b.id from vtocc_a as a left join vtocc_b as b on a.eid = b.eid and a.id = b.id",
			Result: [][]string{
				{"1", "1", "1", "1"},
				{"1", "2", "1", "2"},
			},
			Rewritten: []string{
				"select a.eid, a.id, b.eid, b.id from vtocc_a as a left join vtocc_b as b on 1 != 1 where 1 != 1",
				"select /* left join */ a.eid, a.id, b.eid, b.id from vtocc_a as a left join vtocc_b as b on a.eid = b.eid and a.id = b.id limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "right join",
			Query: "select /* right join */ a.eid, a.id, b.eid, b.id from vtocc_a as a right join vtocc_b as b on a.eid = b.eid and a.id = b.id",
			Result: [][]string{
				{"1", "1", "1", "1"},
				{"1", "2", "1", "2"},
			},
			Rewritten: []string{
				"select a.eid, a.id, b.eid, b.id from vtocc_a as a right join vtocc_b as b on 1 != 1 where 1 != 1",
				"select /* right join */ a.eid, a.id, b.eid, b.id from vtocc_a as a right join vtocc_b as b on a.eid = b.eid and a.id = b.id limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "complex select list",
			Query: "select /* complex select list */ eid+1, id from vtocc_a",
			Result: [][]string{
				{"2", "1"},
				{"2", "2"},
			},
			Rewritten: []string{
				"select eid + 1, id from vtocc_a where 1 != 1",
				"select /* complex select list */ eid + 1, id from vtocc_a limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "*",
			Query: "select /* * */ * from vtocc_a",
			Result: [][]string{
				{"1", "1", "abcd", "efgh"},
				{"1", "2", "bcde", "fghi"},
			},
			Rewritten: []string{
				"select * from vtocc_a where 1 != 1",
				"select /* * */ * from vtocc_a limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "table alias",
			Query: "select /* table alias */ a.eid from vtocc_a as a where a.eid=1",
			Result: [][]string{
				{"1"},
				{"1"},
			},
			Rewritten: []string{
				"select a.eid from vtocc_a as a where 1 != 1",
				"select /* table alias */ a.eid from vtocc_a as a where a.eid = 1 limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "parenthesised col",
			Query: "select /* parenthesised col */ (eid) from vtocc_a where eid = 1 and id = 1",
			Result: [][]string{
				{"1"},
			},
			Rewritten: []string{
				"select (eid) from vtocc_a where 1 != 1",
				"select /* parenthesised col */ (eid) from vtocc_a where eid = 1 and id = 1 limit 10001",
			},
			RowsAffected: 1,
		},
		&framework.MultiCase{
			Name: "for update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "select /* for update */ eid from vtocc_a where eid = 1 and id = 1 for update",
					Result: [][]string{
						{"1"},
					},
					Rewritten: []string{
						"select eid from vtocc_a where 1 != 1",
						"select /* for update */ eid from vtocc_a where eid = 1 and id = 1 limit 10001 for update",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "lock in share mode",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "select /* for update */ eid from vtocc_a where eid = 1 and id = 1 lock in share mode",
					Result: [][]string{
						{"1"},
					},
					Rewritten: []string{
						"select eid from vtocc_a where 1 != 1",
						"select /* for update */ eid from vtocc_a where eid = 1 and id = 1 limit 10001 lock in share mode",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
			},
		},
		&framework.TestCase{
			Name:  "complex where",
			Query: "select /* complex where */ id from vtocc_a where id+1 = 2",
			Result: [][]string{
				{"1"},
			},
			Rewritten: []string{
				"select id from vtocc_a where 1 != 1",
				"select /* complex where */ id from vtocc_a where id + 1 = 2 limit 10001",
			},
			RowsAffected: 1,
		},
		&framework.TestCase{
			Name:  "complex where (non-value operand)",
			Query: "select /* complex where (non-value operand) */ eid, id from vtocc_a where eid = id",
			Result: [][]string{
				{"1", "1"},
			},
			Rewritten: []string{
				"select eid, id from vtocc_a where 1 != 1",
				"select /* complex where (non-value operand) */ eid, id from vtocc_a where eid = id limit 10001",
			},
			RowsAffected: 1,
		},
		&framework.TestCase{
			Name:  "(condition)",
			Query: "select /* (condition) */ * from vtocc_a where (eid = 1)",
			Result: [][]string{
				{"1", "1", "abcd", "efgh"},
				{"1", "2", "bcde", "fghi"},
			},
			Rewritten: []string{
				"select * from vtocc_a where 1 != 1",
				"select /* (condition) */ * from vtocc_a where (eid = 1) limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "inequality",
			Query: "select /* inequality */ * from vtocc_a where id > 1",
			Result: [][]string{
				{"1", "2", "bcde", "fghi"},
			},
			Rewritten: []string{
				"select * from vtocc_a where 1 != 1",
				"select /* inequality */ * from vtocc_a where id > 1 limit 10001",
			},
			RowsAffected: 1,
		},
		&framework.TestCase{
			Name:  "in",
			Query: "select /* in */ * from vtocc_a where id in (1, 2)",
			Result: [][]string{
				{"1", "1", "abcd", "efgh"},
				{"1", "2", "bcde", "fghi"},
			},
			Rewritten: []string{
				"select * from vtocc_a where 1 != 1",
				"select /* in */ * from vtocc_a where id in (1, 2) limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "between",
			Query: "select /* between */ * from vtocc_a where id between 1 and 2",
			Result: [][]string{
				{"1", "1", "abcd", "efgh"},
				{"1", "2", "bcde", "fghi"},
			},
			Rewritten: []string{
				"select * from vtocc_a where 1 != 1",
				"select /* between */ * from vtocc_a where id between 1 and 2 limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "order",
			Query: "select /* order */ * from vtocc_a order by id desc",
			Result: [][]string{
				{"1", "2", "bcde", "fghi"},
				{"1", "1", "abcd", "efgh"},
			},
			Rewritten: []string{
				"select * from vtocc_a where 1 != 1",
				"select /* order */ * from vtocc_a order by id desc limit 10001",
			},
			RowsAffected: 2,
		},
		&framework.TestCase{
			Name:  "select in select list",
			Query: "select (select eid from vtocc_a where id = 1), eid from vtocc_a where id = 2",
			Result: [][]string{
				{"1", "1"},
			},
			Rewritten: []string{
				"select (select eid from vtocc_a where 1 != 1), eid from vtocc_a where 1 != 1",
				"select (select eid from vtocc_a where id = 1), eid from vtocc_a where id = 2 limit 10001",
			},
			RowsAffected: 1,
		},
		&framework.TestCase{
			Name:  "select in from clause",
			Query: "select eid from (select eid from vtocc_a where id=2) as a",
			Result: [][]string{
				{"1"},
			},
			Rewritten: []string{
				"select eid from (select eid from vtocc_a where 1 != 1) as a where 1 != 1",
				"select eid from (select eid from vtocc_a where id = 2) as a limit 10001",
			},
			RowsAffected: 1,
		},
		&framework.MultiCase{
			Name: "select in transaction",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid = 2 and id = 1",
					Rewritten: []string{
						"select * from vtocc_a where 1 != 1",
						"select * from vtocc_a where eid = 2 and id = 1 limit 10001",
					},
				},
				&framework.TestCase{
					Query: "select * from vtocc_a where eid = 2 and id = 1",
					Rewritten: []string{
						"select * from vtocc_a where eid = 2 and id = 1 limit 10001",
					},
				},
				&framework.TestCase{
					Query: "select :bv from vtocc_a where eid = 2 and id = 1",
					BindVars: map[string]interface{}{
						"bv": 1,
					},
					Rewritten: []string{
						"select 1 from vtocc_a where eid = 2 and id = 1 limit 10001",
					},
				},
				&framework.TestCase{
					Query: "select :bv from vtocc_a where eid = 2 and id = 1",
					BindVars: map[string]interface{}{
						"bv": "abcd",
					},
					Rewritten: []string{
						"select 'abcd' from vtocc_a where eid = 2 and id = 1 limit 10001",
					},
				},
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "simple insert",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert /* simple */ into vtocc_a values (2, 1, 'aaaa', 'bbbb')",
					Rewritten: []string{
						"insert /* simple */ into vtocc_a values (2, 1, 'aaaa', 'bbbb') /* _stream vtocc_a (eid id ) (2 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid = 2 and id = 1",
					Result: [][]string{
						{"2", "1", "aaaa", "bbbb"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_a where eid>1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "insert ignore",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert /* simple */ ignore into vtocc_a values (2, 1, 'aaaa', 'bbbb')",
					Rewritten: []string{
						"insert /* simple */ ignore into vtocc_a values (2, 1, 'aaaa', 'bbbb') /* _stream vtocc_a (eid id ) (2 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid = 2 and id = 1",
					Result: [][]string{
						{"2", "1", "aaaa", "bbbb"},
					},
				},
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert /* simple */ ignore into vtocc_a values (2, 1, 'cccc', 'cccc')",
					Rewritten: []string{
						"insert /* simple */ ignore into vtocc_a values (2, 1, 'cccc', 'cccc') /* _stream vtocc_a (eid id ) (2 1 )",
					},
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid = 2 and id = 1",
					Result: [][]string{
						{"2", "1", "aaaa", "bbbb"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_a where eid>1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "qualified insert",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert /* qualified */ into vtocc_a(eid, id, name, foo) values (3, 1, 'aaaa', 'cccc')",
					Rewritten: []string{
						"insert /* qualified */ into vtocc_a(eid, id, name, foo) values (3, 1, 'aaaa', 'cccc') /* _stream vtocc_a (eid id ) (3 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid = 3 and id = 1",
					Result: [][]string{
						{"3", "1", "aaaa", "cccc"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_a where eid>1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "insert with qualified column name",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert /* qualified */ into vtocc_a(vtocc_a.eid, id, name, foo) values (4, 1, 'aaaa', 'cccc')",
					Rewritten: []string{
						"insert /* qualified */ into vtocc_a(vtocc_a.eid, id, name, foo) values (4, 1, 'aaaa', 'cccc') /* _stream vtocc_a (eid id ) (4 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid = 4 and id = 1",
					Result: [][]string{
						{"4", "1", "aaaa", "cccc"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_a where eid>1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "insert auto_increment",
			Cases: []framework.Testable{
				framework.TestQuery("alter table vtocc_e auto_increment = 1"),
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert /* auto_increment */ into vtocc_e(name, foo) values ('aaaa', 'cccc')",
					Rewritten: []string{
						"insert /* auto_increment */ into vtocc_e(name, foo) values ('aaaa', 'cccc') /* _stream vtocc_e (eid id name ) (null 1 'YWFhYQ==' )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_e",
					Result: [][]string{
						{"1", "1", "aaaa", "cccc"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_e"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "insert with number default value",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert /* num default */ into vtocc_a(eid, name, foo) values (3, 'aaaa', 'cccc')",
					Rewritten: []string{
						"insert /* num default */ into vtocc_a(eid, name, foo) values (3, 'aaaa', 'cccc') /* _stream vtocc_a (eid id ) (3 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid = 3 and id = 1",
					Result: [][]string{
						{"3", "1", "aaaa", "cccc"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_a where eid>1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "insert with string default value",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert /* string default */ into vtocc_f(id) values (1)",
					Rewritten: []string{
						"insert /* string default */ into vtocc_f(id) values (1) /* _stream vtocc_f (vb ) ('YWI=' )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_f",
					Result: [][]string{
						{"ab", "1"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_f"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "bind values",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert /* bind values */ into vtocc_a(eid, id, name, foo) values (:eid, :id, :name, :foo)",
					BindVars: map[string]interface{}{
						"foo":  "cccc",
						"eid":  4,
						"name": "aaaa",
						"id":   1,
					},
					Rewritten: []string{
						"insert /* bind values */ into vtocc_a(eid, id, name, foo) values (4, 1, 'aaaa', 'cccc') /* _stream vtocc_a (eid id ) (4 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid = 4 and id = 1",
					Result: [][]string{
						{"4", "1", "aaaa", "cccc"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_a where eid>1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "positional values",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert /* positional values */ into vtocc_a(eid, id, name, foo) values (?, ?, ?, ?)",
					BindVars: map[string]interface{}{
						"v1": 4,
						"v2": 1,
						"v3": "aaaa",
						"v4": "cccc",
					},
					Rewritten: []string{
						"insert /* positional values */ into vtocc_a(eid, id, name, foo) values (4, 1, 'aaaa', 'cccc') /* _stream vtocc_a (eid id ) (4 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid = 4 and id = 1",
					Result: [][]string{
						{"4", "1", "aaaa", "cccc"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_a where eid>1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "out of sequence columns",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into vtocc_a(id, eid, foo, name) values (-1, 5, 'aaa', 'bbb')",
					Rewritten: []string{
						"insert into vtocc_a(id, eid, foo, name) values (-1, 5, 'aaa', 'bbb') /* _stream vtocc_a (eid id ) (5 -1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid = 5 and id = -1",
					Result: [][]string{
						{"5", "-1", "bbb", "aaa"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_a where eid>1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "subquery",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert /* subquery */ into vtocc_a(eid, name, foo) select eid, name, foo from vtocc_c",
					Rewritten: []string{
						"select eid, name, foo from vtocc_c limit 10001",
						"insert /* subquery */ into vtocc_a(eid, name, foo) values (10, 'abcd', '20'), (11, 'bcde', '30') /* _stream vtocc_a (eid id ) (10 1 ) (11 1 )",
					},
					RowsAffected: 2,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid in (10, 11)",
					Result: [][]string{
						{"10", "1", "abcd", "20"},
						{"11", "1", "bcde", "30"},
					},
				},
				framework.TestQuery("alter table vtocc_e auto_increment = 20"),
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into vtocc_e(id, name, foo) select eid, name, foo from vtocc_c",
					Rewritten: []string{
						"select eid, name, foo from vtocc_c limit 10001",
						"insert into vtocc_e(id, name, foo) values (10, 'abcd', '20'), (11, 'bcde', '30') /* _stream vtocc_e (eid id name ) (null 10 'YWJjZA==' ) (null 11 'YmNkZQ==' )",
					},
					RowsAffected: 2,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select eid, id, name, foo from vtocc_e",
					Result: [][]string{
						{"20", "10", "abcd", "20"},
						{"21", "11", "bcde", "30"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_a where eid>1"),
				framework.TestQuery("delete from vtocc_c where eid<10"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "multi-value",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into vtocc_a(eid, id, name, foo) values (5, 1, '', ''), (7, 1, '', '')",
					Rewritten: []string{
						"insert into vtocc_a(eid, id, name, foo) values (5, 1, '', ''), (7, 1, '', '') /* _stream vtocc_a (eid id ) (5 1 ) (7 1 )",
					},
					RowsAffected: 2,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid>1",
					Result: [][]string{
						{"5", "1", "", ""},
						{"7", "1", "", ""},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_a where eid>1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "upsert single row present/absent",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into upsert_test(id1, id2) values (1, 1) on duplicate key update id2 = 1",
					Rewritten: []string{
						"insert into upsert_test(id1, id2) values (1, 1) /* _stream upsert_test (id1 ) (1 )",
					},
					RowsAffected: 1,
				},
				&framework.TestCase{
					Query: "select * from upsert_test",
					Result: [][]string{
						{"1", "1"},
					},
				},
				&framework.TestCase{
					Query: "insert into upsert_test(id1, id2) values (1, 2) on duplicate key update id2 = 2",
					Rewritten: []string{
						"insert into upsert_test(id1, id2) values (1, 2) /* _stream upsert_test (id1 ) (1 )",
						"update upsert_test set id2 = 2 where id1 in (1) /* _stream upsert_test (id1 ) (1 )",
					},
					RowsAffected: 2,
				},
				&framework.TestCase{
					Query: "select * from upsert_test",
					Result: [][]string{
						{"1", "2"},
					},
				},
				&framework.TestCase{
					Query: "insert into upsert_test(id1, id2) values (1, 2) on duplicate key update id2 = 2",
					Rewritten: []string{
						"insert into upsert_test(id1, id2) values (1, 2) /* _stream upsert_test (id1 ) (1 )",
						"update upsert_test set id2 = 2 where id1 in (1) /* _stream upsert_test (id1 ) (1 )",
					},
				},
				&framework.TestCase{
					Query: "insert ignore into upsert_test(id1, id2) values (1, 3) on duplicate key update id2 = 3",
					Rewritten: []string{
						"insert into upsert_test(id1, id2) values (1, 3) /* _stream upsert_test (id1 ) (1 )",
						"update upsert_test set id2 = 3 where id1 in (1) /* _stream upsert_test (id1 ) (1 )",
					},
					RowsAffected: 2,
				},
				&framework.TestCase{
					Query: "select * from upsert_test",
					Result: [][]string{
						{"1", "3"},
					},
				},
				framework.TestQuery("commit"),
				framework.TestQuery("begin"),
				framework.TestQuery("delete from upsert_test"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "upsert changes pk",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into upsert_test(id1, id2) values (1, 1) on duplicate key update id1 = 1",
					Rewritten: []string{
						"insert into upsert_test(id1, id2) values (1, 1) /* _stream upsert_test (id1 ) (1 )",
					},
					RowsAffected: 1,
				},
				&framework.TestCase{
					Query: "select * from upsert_test",
					Result: [][]string{
						{"1", "1"},
					},
				},
				&framework.TestCase{
					Query: "insert into upsert_test(id1, id2) values (1, 2) on duplicate key update id1 = 2",
					Rewritten: []string{
						"insert into upsert_test(id1, id2) values (1, 2) /* _stream upsert_test (id1 ) (1 )",
						"update upsert_test set id1 = 2 where id1 in (1) /* _stream upsert_test (id1 ) (1 ) (2 )",
					},
					RowsAffected: 2,
				},
				&framework.TestCase{
					Query: "select * from upsert_test",
					Result: [][]string{
						{"2", "1"},
					},
				},
				framework.TestQuery("commit"),
				framework.TestQuery("begin"),
				framework.TestQuery("delete from upsert_test"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update /* pk */ vtocc_a set foo='bar' where eid = 1 and id = 1",
					Rewritten: []string{
						"update /* pk */ vtocc_a set foo = 'bar' where (eid = 1 and id = 1) /* _stream vtocc_a (eid id ) (1 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select foo from vtocc_a where id = 1",
					Result: [][]string{
						{"bar"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("update vtocc_a set foo='efgh' where id=1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "single in update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update /* pk */ vtocc_a set foo='bar' where eid = 1 and id in (1, 2)",
					Rewritten: []string{
						"update /* pk */ vtocc_a set foo = 'bar' where (eid = 1 and id = 1) or (eid = 1 and id = 2) /* _stream vtocc_a (eid id ) (1 1 ) (1 2 )",
					},
					RowsAffected: 2,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select foo from vtocc_a where id = 1",
					Result: [][]string{
						{"bar"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("update vtocc_a set foo='efgh' where id=1"),
				framework.TestQuery("update vtocc_a set foo='fghi' where id=2"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "double in update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update /* pk */ vtocc_a set foo='bar' where eid in (1) and id in (1, 2)",
					Rewritten: []string{
						"select eid, id from vtocc_a where eid in (1) and id in (1, 2) limit 10001 for update",
						"update /* pk */ vtocc_a set foo = 'bar' where (eid = 1 and id = 1) or (eid = 1 and id = 2) /* _stream vtocc_a (eid id ) (1 1 ) (1 2 )",
					},
					RowsAffected: 2,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select foo from vtocc_a where id = 1",
					Result: [][]string{
						{"bar"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("update vtocc_a set foo='efgh' where id=1"),
				framework.TestQuery("update vtocc_a set foo='fghi' where id=2"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "double in 2 update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update /* pk */ vtocc_a set foo='bar' where eid in (1, 2) and id in (1, 2)",
					Rewritten: []string{
						"select eid, id from vtocc_a where eid in (1, 2) and id in (1, 2) limit 10001 for update",
						"update /* pk */ vtocc_a set foo = 'bar' where (eid = 1 and id = 1) or (eid = 1 and id = 2) /* _stream vtocc_a (eid id ) (1 1 ) (1 2 )",
					},
					RowsAffected: 2,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select foo from vtocc_a where id = 1",
					Result: [][]string{
						{"bar"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("update vtocc_a set foo='efgh' where id=1"),
				framework.TestQuery("update vtocc_a set foo='fghi' where id=2"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "pk change update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update vtocc_a set eid = 2 where eid = 1 and id = 1",
					Rewritten: []string{
						"update vtocc_a set eid = 2 where (eid = 1 and id = 1) /* _stream vtocc_a (eid id ) (1 1 ) (2 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select eid from vtocc_a where id = 1",
					Result: [][]string{
						{"2"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("update vtocc_a set eid=1 where id=1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "pk change with qualifed column name update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update vtocc_a set vtocc_a.eid = 2 where eid = 1 and id = 1",
					Rewritten: []string{
						"update vtocc_a set vtocc_a.eid = 2 where (eid = 1 and id = 1) /* _stream vtocc_a (eid id ) (1 1 ) (2 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select eid from vtocc_a where id = 1",
					Result: [][]string{
						{"2"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("update vtocc_a set eid=1 where id=1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "partial pk update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update /* pk */ vtocc_a set foo='bar' where id = 1",
					Rewritten: []string{
						"select eid, id from vtocc_a where id = 1 limit 10001 for update",
						"update /* pk */ vtocc_a set foo = 'bar' where (eid = 1 and id = 1) /* _stream vtocc_a (eid id ) (1 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select foo from vtocc_a where id = 1",
					Result: [][]string{
						{"bar"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("update vtocc_a set foo='efgh' where id=1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "limit update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update /* pk */ vtocc_a set foo='bar' where eid = 1 limit 1",
					Rewritten: []string{
						"select eid, id from vtocc_a where eid = 1 limit 1 for update",
						"update /* pk */ vtocc_a set foo = 'bar' where (eid = 1 and id = 1) /* _stream vtocc_a (eid id ) (1 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select foo from vtocc_a where id = 1",
					Result: [][]string{
						{"bar"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("update vtocc_a set foo='efgh' where id=1"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "order by update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update /* pk */ vtocc_a set foo='bar' where eid = 1 order by id desc limit 1",
					Rewritten: []string{
						"select eid, id from vtocc_a where eid = 1 order by id desc limit 1 for update",
						"update /* pk */ vtocc_a set foo = 'bar' where (eid = 1 and id = 2) /* _stream vtocc_a (eid id ) (1 2 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select foo from vtocc_a where id = 2",
					Result: [][]string{
						{"bar"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("update vtocc_a set foo='fghi' where id=2"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "missing where update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update vtocc_a set foo='bar'",
					Rewritten: []string{
						"select eid, id from vtocc_a limit 10001 for update",
						"update vtocc_a set foo = 'bar' where (eid = 1 and id = 1) or (eid = 1 and id = 2) /* _stream vtocc_a (eid id ) (1 1 ) (1 2 )",
					},
					RowsAffected: 2,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a",
					Result: [][]string{
						{"1", "1", "abcd", "bar"},
						{"1", "2", "bcde", "bar"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("update vtocc_a set foo='efgh' where id=1"),
				framework.TestQuery("update vtocc_a set foo='fghi' where id=2"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "single pk update one row update",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				framework.TestQuery("insert into vtocc_f(vb,id) values ('a', 1), ('b', 2)"),
				framework.TestQuery("commit"),
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update vtocc_f set id=2 where vb='a'",
					Rewritten: []string{
						"update vtocc_f set id = 2 where vb in ('a') /* _stream vtocc_f (vb ) ('YQ==' )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_f",
					Result: [][]string{
						{"a", "2"},
						{"b", "2"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_f"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "single pk update two rows",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				framework.TestQuery("insert into vtocc_f(vb,id) values ('a', 1), ('b', 2)"),
				framework.TestQuery("commit"),
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update vtocc_f set id=3 where vb in ('a', 'b')",
					Rewritten: []string{
						"update vtocc_f set id = 3 where vb in ('a', 'b') /* _stream vtocc_f (vb ) ('YQ==' ) ('Yg==' )",
					},
					RowsAffected: 2,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_f",
					Result: [][]string{
						{"a", "3"},
						{"b", "3"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_f"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "single pk update subquery",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				framework.TestQuery("insert into vtocc_f(vb,id) values ('a', 1), ('b', 2)"),
				framework.TestQuery("commit"),
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update vtocc_f set id=4 where id >= 0",
					Rewritten: []string{
						"select vb from vtocc_f where id >= 0 limit 10001 for update",
						"update vtocc_f set id = 4 where vb in ('a', 'b') /* _stream vtocc_f (vb ) ('YQ==' ) ('Yg==' )",
					},
					RowsAffected: 2,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_f",
					Result: [][]string{
						{"a", "4"},
						{"b", "4"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_f"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "single pk update subquery no rows",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				framework.TestQuery("insert into vtocc_f(vb,id) values ('a', 1), ('b', 2)"),
				framework.TestQuery("commit"),
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "update vtocc_f set id=4 where id < 0",
					Rewritten: []string{
						"select vb from vtocc_f where id < 0 limit 10001 for update",
					},
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_f",
					Result: [][]string{
						{"a", "1"},
						{"b", "2"},
					},
				},
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_f"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "delete",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				framework.TestQuery("insert into vtocc_a(eid, id, name, foo) values (2, 1, '', '')"),
				&framework.TestCase{
					Query: "delete /* pk */ from vtocc_a where eid = 2 and id = 1",
					Rewritten: []string{
						"delete /* pk */ from vtocc_a where (eid = 2 and id = 1) /* _stream vtocc_a (eid id ) (2 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid=2",
				},
			},
		},
		&framework.MultiCase{
			Name: "single in delete",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				framework.TestQuery("insert into vtocc_a(eid, id, name, foo) values (2, 1, '', '')"),
				&framework.TestCase{
					Query: "delete /* pk */ from vtocc_a where eid = 2 and id in (1, 2)",
					Rewritten: []string{
						"delete /* pk */ from vtocc_a where (eid = 2 and id = 1) or (eid = 2 and id = 2) /* _stream vtocc_a (eid id ) (2 1 ) (2 2 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid=2",
				},
			},
		},
		&framework.MultiCase{
			Name: "double in delete",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				framework.TestQuery("insert into vtocc_a(eid, id, name, foo) values (2, 1, '', '')"),
				&framework.TestCase{
					Query: "delete /* pk */ from vtocc_a where eid in (2) and id in (1, 2)",
					Rewritten: []string{
						"select eid, id from vtocc_a where eid in (2) and id in (1, 2) limit 10001 for update",
						"delete /* pk */ from vtocc_a where (eid = 2 and id = 1) /* _stream vtocc_a (eid id ) (2 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid=2",
				},
			},
		},
		&framework.MultiCase{
			Name: "double in 2 delete",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				framework.TestQuery("insert into vtocc_a(eid, id, name, foo) values (2, 1, '', '')"),
				&framework.TestCase{
					Query: "delete /* pk */ from vtocc_a where eid in (2, 3) and id in (1, 2)",
					Rewritten: []string{
						"select eid, id from vtocc_a where eid in (2, 3) and id in (1, 2) limit 10001 for update",
						"delete /* pk */ from vtocc_a where (eid = 2 and id = 1) /* _stream vtocc_a (eid id ) (2 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid=2",
				},
			},
		},
		&framework.MultiCase{
			Name: "complex where delete",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				framework.TestQuery("insert into vtocc_a(eid, id, name, foo) values (2, 1, '', '')"),
				&framework.TestCase{
					Query: "delete from vtocc_a where eid = 1+1 and id = 1",
					Rewritten: []string{
						"select eid, id from vtocc_a where eid = 1 + 1 and id = 1 limit 10001 for update",
						"delete from vtocc_a where (eid = 2 and id = 1) /* _stream vtocc_a (eid id ) (2 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid=2",
				},
			},
		},
		&framework.MultiCase{
			Name: "partial pk delete",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				framework.TestQuery("insert into vtocc_a(eid, id, name, foo) values (2, 1, '', '')"),
				&framework.TestCase{
					Query: "delete from vtocc_a where eid = 2",
					Rewritten: []string{
						"select eid, id from vtocc_a where eid = 2 limit 10001 for update",
						"delete from vtocc_a where (eid = 2 and id = 1) /* _stream vtocc_a (eid id ) (2 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid=2",
				},
			},
		},
		&framework.MultiCase{
			Name: "limit delete",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				framework.TestQuery("insert into vtocc_a(eid, id, name, foo) values (2, 1, '', '')"),
				&framework.TestCase{
					Query: "delete from vtocc_a where eid = 2 limit 1",
					Rewritten: []string{
						"select eid, id from vtocc_a where eid = 2 limit 1 for update",
						"delete from vtocc_a where (eid = 2 and id = 1) /* _stream vtocc_a (eid id ) (2 1 )",
					},
					RowsAffected: 1,
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_a where eid=2",
				},
			},
		},
		&framework.MultiCase{
			Name: "integer data types",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into vtocc_ints values(:tiny, :tinyu, :small, :smallu, :medium, :mediumu, :normal, :normalu, :big, :bigu, :year)",
					BindVars: map[string]interface{}{
						"medium":  -8388608,
						"smallu":  65535,
						"normal":  -2147483648,
						"big":     -9223372036854775808,
						"tinyu":   255,
						"year":    2012,
						"tiny":    -128,
						"bigu":    uint64(18446744073709551615),
						"normalu": 4294967295,
						"small":   -32768,
						"mediumu": 16777215,
					},
					Rewritten: []string{
						"insert into vtocc_ints values (-128, 255, -32768, 65535, -8388608, 16777215, -2147483648, 4294967295, -9223372036854775808, 18446744073709551615, 2012) /* _stream vtocc_ints (tiny ) (-128 )",
					},
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_ints where tiny = -128",
					Result: [][]string{
						{"-128", "255", "-32768", "65535", "-8388608", "16777215", "-2147483648", "4294967295", "-9223372036854775808", "18446744073709551615", "2012"},
					},
					Rewritten: []string{
						"select * from vtocc_ints where 1 != 1",
						"select * from vtocc_ints where tiny = -128 limit 10001",
					},
				},
				&framework.TestCase{
					Query: "select * from vtocc_ints where tiny = -128",
					Result: [][]string{
						{"-128", "255", "-32768", "65535", "-8388608", "16777215", "-2147483648", "4294967295", "-9223372036854775808", "18446744073709551615", "2012"},
					},
					Rewritten: []string{
						"select * from vtocc_ints where tiny = -128 limit 10001",
					},
				},
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into vtocc_ints select 2, tinyu, small, smallu, medium, mediumu, normal, normalu, big, bigu, y from vtocc_ints",
					Rewritten: []string{
						"select 2, tinyu, small, smallu, medium, mediumu, normal, normalu, big, bigu, y from vtocc_ints limit 10001",
						"insert into vtocc_ints values (2, 255, -32768, 65535, -8388608, 16777215, -2147483648, 4294967295, -9223372036854775808, 18446744073709551615, 2012) /* _stream vtocc_ints (tiny ) (2 )",
					},
				},
				framework.TestQuery("commit"),
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_ints"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "fractional data types",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into vtocc_fracts values(:id, :deci, :num, :f, :d)",
					BindVars: map[string]interface{}{
						"d":    4.99,
						"num":  "2.99",
						"id":   1,
						"f":    3.99,
						"deci": "1.99",
					},
					Rewritten: []string{
						"insert into vtocc_fracts values (1, '1.99', '2.99', 3.99, 4.99) /* _stream vtocc_fracts (id ) (1 )",
					},
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_fracts where id = 1",
					Result: [][]string{
						{"1", "1.99", "2.99", "3.99", "4.99"},
					},
					Rewritten: []string{
						"select * from vtocc_fracts where 1 != 1",
						"select * from vtocc_fracts where id = 1 limit 10001",
					},
				},
				&framework.TestCase{
					Query: "select * from vtocc_fracts where id = 1",
					Result: [][]string{
						{"1", "1.99", "2.99", "3.99", "4.99"},
					},
					Rewritten: []string{
						"select * from vtocc_fracts where id = 1 limit 10001",
					},
				},
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into vtocc_fracts select 2, deci, num, f, d from vtocc_fracts",
					Rewritten: []string{
						"select 2, deci, num, f, d from vtocc_fracts limit 10001",
						"insert into vtocc_fracts values (2, 1.99, 2.99, 3.99, 4.99) /* _stream vtocc_fracts (id ) (2 )",
					},
				},
				framework.TestQuery("commit"),
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_fracts"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "string data types",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into vtocc_strings values (:vb, :c, :vc, :b, :tb, :bl, :ttx, :tx, :en, :s)",
					BindVars: map[string]interface{}{
						"ttx": "g",
						"vb":  "a",
						"vc":  "c",
						"en":  "a",
						"tx":  "h",
						"bl":  "f",
						"s":   "a,b",
						"b":   "d",
						"tb":  "e",
						"c":   "b",
					},
					Rewritten: []string{
						"insert into vtocc_strings values ('a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'a', 'a,b') /* _stream vtocc_strings (vb ) ('YQ==' )",
					},
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_strings where vb = 'a'",
					Result: [][]string{
						{"a", "b", "c", "d\x00\x00\x00", "e", "f", "g", "h", "a", "a,b"},
					},
					Rewritten: []string{
						"select * from vtocc_strings where 1 != 1",
						"select * from vtocc_strings where vb = 'a' limit 10001",
					},
				},
				&framework.TestCase{
					Query: "select * from vtocc_strings where vb = 'a'",
					Result: [][]string{
						{"a", "b", "c", "d\x00\x00\x00", "e", "f", "g", "h", "a", "a,b"},
					},
					Rewritten: []string{
						"select * from vtocc_strings where vb = 'a' limit 10001",
					},
				},
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into vtocc_strings select 'b', c, vc, b, tb, bl, ttx, tx, en, s from vtocc_strings",
					Rewritten: []string{
						"select 'b', c, vc, b, tb, bl, ttx, tx, en, s from vtocc_strings limit 10001",
						"insert into vtocc_strings values ('b', 'b', 'c', 'd\\0\\0\\0', 'e', 'f', 'g', 'h', 'a', 'a,b') /* _stream vtocc_strings (vb ) ('Yg==' )",
					},
				},
				framework.TestQuery("commit"),
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_strings"),
				framework.TestQuery("commit"),
			},
		},
		&framework.MultiCase{
			Name: "misc data types",
			Cases: []framework.Testable{
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into vtocc_misc values(:id, :b, :d, :dt, :t)",
					BindVars: map[string]interface{}{
						"t":  "15:45:45",
						"dt": "2012-01-01 15:45:45",
						"b":  "\x01",
						"id": 1,
						"d":  "2012-01-01",
					},
					Rewritten: []string{
						"insert into vtocc_misc values (1, '\x01', '2012-01-01', '2012-01-01 15:45:45', '15:45:45') /* _stream vtocc_misc (id ) (1 )",
					},
				},
				framework.TestQuery("commit"),
				&framework.TestCase{
					Query: "select * from vtocc_misc where id = 1",
					Result: [][]string{
						{"1", "\x01", "2012-01-01", "2012-01-01 15:45:45", "15:45:45"},
					},
					Rewritten: []string{
						"select * from vtocc_misc where 1 != 1",
						"select * from vtocc_misc where id = 1 limit 10001",
					},
				},
				&framework.TestCase{
					Query: "select * from vtocc_misc where id = 1",
					Result: [][]string{
						{"1", "\x01", "2012-01-01", "2012-01-01 15:45:45", "15:45:45"},
					},
					Rewritten: []string{
						"select * from vtocc_misc where id = 1 limit 10001",
					},
				},
				framework.TestQuery("begin"),
				&framework.TestCase{
					Query: "insert into vtocc_misc select 2, b, d, dt, t from vtocc_misc",
					Rewritten: []string{
						"select 2, b, d, dt, t from vtocc_misc limit 10001",
						"insert into vtocc_misc values (2, '\x01', '2012-01-01', '2012-01-01 15:45:45', '15:45:45') /* _stream vtocc_misc (id ) (2 )",
					},
				},
				framework.TestQuery("commit"),
				framework.TestQuery("begin"),
				framework.TestQuery("delete from vtocc_misc"),
				framework.TestQuery("commit"),
			},
		},
	}
	for _, tcase := range testCases {
		if err := tcase.Test("", client); err != nil {
			t.Error(err)
		}
	}
}
