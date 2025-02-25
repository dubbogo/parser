// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package ast_test

import (
	"fmt"
	"testing"

	"github.com/arana-db/parser"
	"github.com/arana-db/parser/ast"
	"github.com/arana-db/parser/auth"
	"github.com/arana-db/parser/mysql"
	"github.com/stretchr/testify/require"
)

type visitor struct{}

func (v visitor) Enter(in ast.Node) (ast.Node, bool) {
	return in, false
}

func (v visitor) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

type visitor1 struct {
	visitor
}

func (visitor1) Enter(in ast.Node) (ast.Node, bool) {
	return in, true
}

func TestMiscVisitorCover(t *testing.T) {
	valueExpr := ast.NewValueExpr(42, mysql.DefaultCharset, mysql.DefaultCollationName)
	stmts := []ast.Node{
		&ast.AdminStmt{},
		&ast.AlterUserStmt{},
		&ast.BeginStmt{},
		&ast.BinlogStmt{},
		&ast.CommitStmt{},
		&ast.CreateUserStmt{},
		&ast.DeallocateStmt{},
		&ast.DoStmt{},
		&ast.ExecuteStmt{UsingVars: []ast.ExprNode{valueExpr}},
		&ast.ExplainStmt{Stmt: &ast.ShowStmt{}},
		&ast.GrantStmt{},
		&ast.PrepareStmt{SQLVar: &ast.VariableExpr{Value: valueExpr}},
		&ast.RollbackStmt{},
		&ast.SetPwdStmt{},
		&ast.SetStmt{Variables: []*ast.VariableAssignment{
			{
				Value: valueExpr,
			},
		}},
		&ast.UseStmt{},
		&ast.AnalyzeTableStmt{
			TableNames: []*ast.TableName{
				{},
			},
		},
		&ast.FlushStmt{},
		&ast.PrivElem{},
		&ast.VariableAssignment{Value: valueExpr},
		&ast.KillStmt{},
		&ast.DropStatsStmt{Table: &ast.TableName{}},
		&ast.ShutdownStmt{},
	}

	for _, v := range stmts {
		v.Accept(visitor{})
		v.Accept(visitor1{})
	}
}

func TestDDLVisitorCoverMisc(t *testing.T) {
	sql := `
create table t (c1 smallint unsigned, c2 int unsigned);
alter table t add column a smallint unsigned after b;
alter table t add column (a int, constraint check (a > 0));
create index t_i on t (id);
create database test character set utf8;
drop database test;
drop index t_i on t;
drop table t;
truncate t;
create table t (
jobAbbr char(4) not null,
constraint foreign key (jobabbr) references ffxi_jobtype (jobabbr) on delete cascade on update cascade
);
`
	parse := parser.New()
	stmts, _, err := parse.Parse(sql, "", "")
	require.NoError(t, err)
	for _, stmt := range stmts {
		stmt.Accept(visitor{})
		stmt.Accept(visitor1{})
	}
}

func TestDMLVistorCover(t *testing.T) {
	sql := `delete from somelog where user = 'jcole' order by timestamp_column limit 1;
delete t1, t2 from t1 inner join t2 inner join t3 where t1.id=t2.id and t2.id=t3.id;
select * from t where exists(select * from t k where t.c = k.c having sum(c) = 1);
insert into t_copy select * from t where t.x > 5;
(select /*+ TIDB_INLJ(t1) */ a from t1 where a=10 and b=1) union (select /*+ TIDB_SMJ(t2) */ a from t2 where a=11 and b=2) order by a limit 10;
update t1 set col1 = col1 + 1, col2 = col1;
show create table t;
load data infile '/tmp/t.csv' into table t fields terminated by 'ab' enclosed by 'b';`

	p := parser.New()
	stmts, _, err := p.Parse(sql, "", "")
	require.NoError(t, err)
	for _, stmt := range stmts {
		stmt.Accept(visitor{})
		stmt.Accept(visitor1{})
	}
}

// test Change Pump or drainer status sql parser
func TestChangeStmt(t *testing.T) {
	sql := `change pump to node_state='paused' for node_id '127.0.0.1:8249';
change drainer to node_state='paused' for node_id '127.0.0.1:8249';
shutdown;`

	p := parser.New()
	stmts, _, err := p.Parse(sql, "", "")
	require.NoError(t, err)
	for _, stmt := range stmts {
		stmt.Accept(visitor{})
		stmt.Accept(visitor1{})
	}
}

func TestSensitiveStatement(t *testing.T) {
	positive := []ast.StmtNode{
		&ast.SetPwdStmt{},
		&ast.CreateUserStmt{},
		&ast.AlterUserStmt{},
		&ast.GrantStmt{},
	}
	for i, stmt := range positive {
		_, ok := stmt.(ast.SensitiveStmtNode)
		require.Truef(t, ok, "%d, %#v fail", i, stmt)
	}

	negative := []ast.StmtNode{
		&ast.DropUserStmt{},
		&ast.RevokeStmt{},
		&ast.AlterTableStmt{},
		&ast.CreateDatabaseStmt{},
		&ast.CreateIndexStmt{},
		&ast.CreateTableStmt{},
		&ast.DropDatabaseStmt{},
		&ast.DropIndexStmt{},
		&ast.DropTableStmt{},
		&ast.RenameTableStmt{},
		&ast.TruncateTableStmt{},
	}
	for _, stmt := range negative {
		_, ok := stmt.(ast.SensitiveStmtNode)
		require.False(t, ok)
	}
}

func TestUserSpec(t *testing.T) {
	hashString := "*3D56A309CD04FA2EEF181462E59011F075C89548"
	u := ast.UserSpec{
		User: &auth.UserIdentity{
			Username: "test",
		},
		AuthOpt: &ast.AuthOption{
			ByAuthString: false,
			AuthString:   "xxx",
			HashString:   hashString,
		},
	}
	pwd, ok := u.EncodedPassword()
	require.True(t, ok)
	require.Equal(t, u.AuthOpt.HashString, pwd)

	u.AuthOpt.HashString = "not-good-password-format"
	_, ok = u.EncodedPassword()
	require.False(t, ok)

	u.AuthOpt.ByAuthString = true
	pwd, ok = u.EncodedPassword()
	require.True(t, ok)
	require.Equal(t, hashString, pwd)

	u.AuthOpt.AuthString = ""
	pwd, ok = u.EncodedPassword()
	require.True(t, ok)
	require.Equal(t, "", pwd)
}

func TestTableOptimizerHintRestore(t *testing.T) {
	testCases := []NodeRestoreTestCase{
		// mysql 8.0 table level
		{"BKA()", "BKA()"},
		{"BKA(@qb1)", "BKA(@`qb1` )"},
		{"BKA(tbl1)", "BKA(`tbl1`)"},
		{"BKA(tbl1@qb1,tbl2@qb2)", "BKA(`tbl1`@`qb1`, `tbl2`@`qb2`)"},
		{"NO_BKA()", "NO_BKA()"},
		{"NO_BKA(@qb1)", "NO_BKA(@`qb1` )"},
		{"NO_BKA(tbl1)", "NO_BKA(`tbl1`)"},
		{"NO_BKA(tbl1@qb1,tbl2@qb2)", "NO_BKA(`tbl1`@`qb1`, `tbl2`@`qb2`)"},
		{"BNL()", "BNL()"},
		{"BNL(@qb1)", "BNL(@`qb1` )"},
		{"BNL(tbl1)", "BNL(`tbl1`)"},
		{"BNL(tbl1@qb1,tbl2@qb2)", "BNL(`tbl1`@`qb1`, `tbl2`@`qb2`)"},
		{"NO_BNL()", "NO_BNL()"},
		{"NO_BNL(@qb1)", "NO_BNL(@`qb1` )"},
		{"NO_BNL(tbl1)", "NO_BNL(`tbl1`)"},
		{"NO_BNL(tbl1@qb1,tbl2@qb2)", "NO_BNL(`tbl1`@`qb1`, `tbl2`@`qb2`)"},
		{"DERIVED_CONDITION_PUSHDOWN()", "DERIVED_CONDITION_PUSHDOWN()"},
		{"DERIVED_CONDITION_PUSHDOWN(@qb1)", "DERIVED_CONDITION_PUSHDOWN(@`qb1` )"},
		{"DERIVED_CONDITION_PUSHDOWN(tbl1)", "DERIVED_CONDITION_PUSHDOWN(`tbl1`)"},
		{"DERIVED_CONDITION_PUSHDOWN(tbl1@qb1,tbl2@qb2)", "DERIVED_CONDITION_PUSHDOWN(`tbl1`@`qb1`, `tbl2`@`qb2`)"},
		{"NO_DERIVED_CONDITION_PUSHDOWN()", "NO_DERIVED_CONDITION_PUSHDOWN()"},
		{"NO_DERIVED_CONDITION_PUSHDOWN(@qb1)", "NO_DERIVED_CONDITION_PUSHDOWN(@`qb1` )"},
		{"NO_DERIVED_CONDITION_PUSHDOWN(tbl1)", "NO_DERIVED_CONDITION_PUSHDOWN(`tbl1`)"},
		{"NO_DERIVED_CONDITION_PUSHDOWN(tbl1@qb1,tbl2@qb2)", "NO_DERIVED_CONDITION_PUSHDOWN(`tbl1`@`qb1`, `tbl2`@`qb2`)"},
		{"HASH_JOIN()", "HASH_JOIN()"},
		{"HASH_JOIN(@qb1)", "HASH_JOIN(@`qb1` )"},
		{"HASH_JOIN(tbl1)", "HASH_JOIN(`tbl1`)"},
		{"HASH_JOIN(tbl1@qb1,tbl2@qb2)", "HASH_JOIN(`tbl1`@`qb1`, `tbl2`@`qb2`)"},
		{"NO_HASH_JOIN()", "NO_HASH_JOIN()"},
		{"NO_HASH_JOIN(@qb1)", "NO_HASH_JOIN(@`qb1` )"},
		{"NO_HASH_JOIN(tbl1)", "NO_HASH_JOIN(`tbl1`)"},
		{"NO_HASH_JOIN(tbl1@qb1,tbl2@qb2)", "NO_HASH_JOIN(`tbl1`@`qb1`, `tbl2`@`qb2`)"},
		{"JOIN_FIXED_ORDER()", "JOIN_FIXED_ORDER()"},
		{"JOIN_FIXED_ORDER(@qb1)", "JOIN_FIXED_ORDER(@`qb1` )"},
		{"JOIN_FIXED_ORDER(tbl1)", "JOIN_FIXED_ORDER(`tbl1`)"},
		{"JOIN_FIXED_ORDER(tbl1@qb1,tbl2@qb2)", "JOIN_FIXED_ORDER(`tbl1`@`qb1`, `tbl2`@`qb2`)"},
		// mysql 8.0 index level
		{"GROUP_INDEX(t1 c1)", "GROUP_INDEX(`t1` `c1`)"},
		{"GROUP_INDEX(@sel1 t1 c1)", "GROUP_INDEX(@`sel1` `t1` `c1`)"},
		{"GROUP_INDEX(t1@sel1 c1)", "GROUP_INDEX(`t1`@`sel1` `c1`)"},
		{"NO_GROUP_INDEX(t1 c1)", "NO_GROUP_INDEX(`t1` `c1`)"},
		{"NO_GROUP_INDEX(@sel1 t1 c1)", "NO_GROUP_INDEX(@`sel1` `t1` `c1`)"},
		{"NO_GROUP_INDEX(t1@sel1 c1)", "NO_GROUP_INDEX(`t1`@`sel1` `c1`)"},
		{"INDEX(t1 c1)", "INDEX(`t1` `c1`)"},
		{"INDEX(@sel1 t1 c1)", "INDEX(@`sel1` `t1` `c1`)"},
		{"INDEX(t1@sel1 c1)", "INDEX(`t1`@`sel1` `c1`)"},
		{"NO_INDEX(t1@sel1 c1)", "NO_INDEX(`t1`@`sel1` `c1`)"},
		{"NO_INDEX(@sel1 t1 c1)", "NO_INDEX(@`sel1` `t1` `c1`)"},
		{"NO_INDEX(t1@sel1 c1)", "NO_INDEX(`t1`@`sel1` `c1`)"},
		{"JOIN_INDEX(t1 c1)", "JOIN_INDEX(`t1` `c1`)"},
		{"JOIN_INDEX(@sel1 t1 c1)", "JOIN_INDEX(@`sel1` `t1` `c1`)"},
		{"JOIN_INDEX(t1@sel1 c1)", "JOIN_INDEX(`t1`@`sel1` `c1`)"},
		{"NO_JOIN_INDEX(t1 c1)", "NO_JOIN_INDEX(`t1` `c1`)"},
		{"NO_JOIN_INDEX(@sel1 t1 c1)", "NO_JOIN_INDEX(@`sel1` `t1` `c1`)"},
		{"NO_JOIN_INDEX(t1@sel1 c1)", "NO_JOIN_INDEX(`t1`@`sel1` `c1`)"},
		{"INDEX_MERGE(t1 c1)", "INDEX_MERGE(`t1` `c1`)"},
		{"INDEX_MERGE(@sel1 t1 c1)", "INDEX_MERGE(@`sel1` `t1` `c1`)"},
		{"INDEX_MERGE(t1@sel1 c1)", "INDEX_MERGE(`t1`@`sel1` `c1`)"},
		{"NO_INDEX_MERGE(t1 c1)", "NO_INDEX_MERGE(`t1` `c1`)"},
		{"NO_INDEX_MERGE(@sel1 t1 c1)", "NO_INDEX_MERGE(@`sel1` `t1` `c1`)"},
		{"NO_INDEX_MERGE(t1@sel1 c1)", "NO_INDEX_MERGE(`t1`@`sel1` `c1`)"},

		{"USE_INDEX(t1 c1)", "USE_INDEX(`t1` `c1`)"},
		{"USE_INDEX(test.t1 c1)", "USE_INDEX(`test`.`t1` `c1`)"},
		{"USE_INDEX(@sel_1 t1 c1)", "USE_INDEX(@`sel_1` `t1` `c1`)"},
		{"USE_INDEX(t1@sel_1 c1)", "USE_INDEX(`t1`@`sel_1` `c1`)"},
		{"USE_INDEX(test.t1@sel_1 c1)", "USE_INDEX(`test`.`t1`@`sel_1` `c1`)"},
		{"USE_INDEX(test.t1@sel_1 partition(p0) c1)", "USE_INDEX(`test`.`t1`@`sel_1` PARTITION(`p0`) `c1`)"},
		{"FORCE_INDEX(t1 c1)", "FORCE_INDEX(`t1` `c1`)"},
		{"FORCE_INDEX(test.t1 c1)", "FORCE_INDEX(`test`.`t1` `c1`)"},
		{"FORCE_INDEX(@sel_1 t1 c1)", "FORCE_INDEX(@`sel_1` `t1` `c1`)"},
		{"FORCE_INDEX(t1@sel_1 c1)", "FORCE_INDEX(`t1`@`sel_1` `c1`)"},
		{"FORCE_INDEX(test.t1@sel_1 c1)", "FORCE_INDEX(`test`.`t1`@`sel_1` `c1`)"},
		{"FORCE_INDEX(test.t1@sel_1 partition(p0) c1)", "FORCE_INDEX(`test`.`t1`@`sel_1` PARTITION(`p0`) `c1`)"},
		{"IGNORE_INDEX(t1 c1)", "IGNORE_INDEX(`t1` `c1`)"},
		{"IGNORE_INDEX(@sel_1 t1 c1)", "IGNORE_INDEX(@`sel_1` `t1` `c1`)"},
		{"IGNORE_INDEX(t1@sel_1 c1)", "IGNORE_INDEX(`t1`@`sel_1` `c1`)"},
		{"IGNORE_INDEX(t1@sel_1 partition(p0, p1) c1)", "IGNORE_INDEX(`t1`@`sel_1` PARTITION(`p0`, `p1`) `c1`)"},
		{"TIDB_SMJ(`t1`)", "TIDB_SMJ(`t1`)"},
		{"TIDB_SMJ(t1)", "TIDB_SMJ(`t1`)"},
		{"TIDB_SMJ(t1,t2)", "TIDB_SMJ(`t1`, `t2`)"},
		{"TIDB_SMJ(@sel1 t1,t2)", "TIDB_SMJ(@`sel1` `t1`, `t2`)"},
		{"TIDB_SMJ(t1@sel1,t2@sel2)", "TIDB_SMJ(`t1`@`sel1`, `t2`@`sel2`)"},
		{"TIDB_INLJ(t1,t2)", "TIDB_INLJ(`t1`, `t2`)"},
		{"TIDB_INLJ(@sel1 t1,t2)", "TIDB_INLJ(@`sel1` `t1`, `t2`)"},
		{"TIDB_INLJ(t1@sel1,t2@sel2)", "TIDB_INLJ(`t1`@`sel1`, `t2`@`sel2`)"},
		{"TIDB_HJ(t1,t2)", "TIDB_HJ(`t1`, `t2`)"},
		{"TIDB_HJ(@sel1 t1,t2)", "TIDB_HJ(@`sel1` `t1`, `t2`)"},
		{"TIDB_HJ(t1@sel1,t2@sel2)", "TIDB_HJ(`t1`@`sel1`, `t2`@`sel2`)"},
		{"MERGE_JOIN(t1,t2)", "MERGE_JOIN(`t1`, `t2`)"},
		{"BROADCAST_JOIN(t1,t2)", "BROADCAST_JOIN(`t1`, `t2`)"},
		{"INL_HASH_JOIN(t1,t2)", "INL_HASH_JOIN(`t1`, `t2`)"},
		{"INL_MERGE_JOIN(t1,t2)", "INL_MERGE_JOIN(`t1`, `t2`)"},
		{"INL_JOIN(t1,t2)", "INL_JOIN(`t1`, `t2`)"},
		{"MAX_EXECUTION_TIME(3000)", "MAX_EXECUTION_TIME(3000)"},
		{"MAX_EXECUTION_TIME(@sel1 3000)", "MAX_EXECUTION_TIME(@`sel1` 3000)"},
		{"USE_INDEX_MERGE(t1 c1)", "USE_INDEX_MERGE(`t1` `c1`)"},
		{"USE_INDEX_MERGE(@sel1 t1 c1)", "USE_INDEX_MERGE(@`sel1` `t1` `c1`)"},
		{"USE_INDEX_MERGE(t1@sel1 c1)", "USE_INDEX_MERGE(`t1`@`sel1` `c1`)"},
		{"USE_TOJA(TRUE)", "USE_TOJA(TRUE)"},
		{"USE_TOJA(FALSE)", "USE_TOJA(FALSE)"},
		{"USE_TOJA(@sel1 TRUE)", "USE_TOJA(@`sel1` TRUE)"},
		{"USE_CASCADES(TRUE)", "USE_CASCADES(TRUE)"},
		{"USE_CASCADES(FALSE)", "USE_CASCADES(FALSE)"},
		{"USE_CASCADES(@sel1 TRUE)", "USE_CASCADES(@`sel1` TRUE)"},
		{"QUERY_TYPE(OLAP)", "QUERY_TYPE(OLAP)"},
		{"QUERY_TYPE(OLTP)", "QUERY_TYPE(OLTP)"},
		{"QUERY_TYPE(@sel1 OLTP)", "QUERY_TYPE(@`sel1` OLTP)"},
		{"NTH_PLAN(10)", "NTH_PLAN(10)"},
		{"NTH_PLAN(@sel1 30)", "NTH_PLAN(@`sel1` 30)"},
		{"MEMORY_QUOTA(1 GB)", "MEMORY_QUOTA(1024 MB)"},
		{"MEMORY_QUOTA(@sel1 1 GB)", "MEMORY_QUOTA(@`sel1` 1024 MB)"},
		{"HASH_AGG()", "HASH_AGG()"},
		{"HASH_AGG(@sel1)", "HASH_AGG(@`sel1`)"},
		{"STREAM_AGG()", "STREAM_AGG()"},
		{"STREAM_AGG(@sel1)", "STREAM_AGG(@`sel1`)"},
		{"AGG_TO_COP()", "AGG_TO_COP()"},
		{"AGG_TO_COP(@sel_1)", "AGG_TO_COP(@`sel_1`)"},
		{"LIMIT_TO_COP()", "LIMIT_TO_COP()"},
		{"READ_CONSISTENT_REPLICA()", "READ_CONSISTENT_REPLICA()"},
		{"READ_CONSISTENT_REPLICA(@sel1)", "READ_CONSISTENT_REPLICA(@`sel1`)"},
		{"QB_NAME(sel1)", "QB_NAME(`sel1`)"},
		{"READ_FROM_STORAGE(@sel TIFLASH[t1, t2])", "READ_FROM_STORAGE(@`sel` TIFLASH[`t1`, `t2`])"},
		{"READ_FROM_STORAGE(@sel TIFLASH[t1 partition(p0)])", "READ_FROM_STORAGE(@`sel` TIFLASH[`t1` PARTITION(`p0`)])"},
		{"TIME_RANGE('2020-02-02 10:10:10','2020-02-02 11:10:10')", "TIME_RANGE('2020-02-02 10:10:10', '2020-02-02 11:10:10')"},
		{"JOIN_PREFIX(t2, t5@subq2)", "JOIN_PREFIX(`t2`, `t5`@`subq2`)"},
		{"JOIN_PREFIX(@subq2 t2, t5)", "JOIN_PREFIX(@`subq2` `t2`, `t5`)"},
		{"JOIN_ORDER(t2, t5@subq2)", "JOIN_ORDER(`t2`, `t5`@`subq2`)"},
		{"JOIN_ORDER(@subq2 t2, t5)", "JOIN_ORDER(@`subq2` `t2`, `t5`)"},
		{"JOIN_SUFFIX(t2, t5@subq2)", "JOIN_SUFFIX(`t2`, `t5`@`subq2`)"},
		{"JOIN_SUFFIX(@subq2 t2, t5)", "JOIN_SUFFIX(@`subq2` `t2`, `t5`)"},
		{"MERGE(t2, t5@subq2)", "MERGE(`t2`, `t5`@`subq2`)"},
		{"MERGE(@subq2 t2, t5)", "MERGE(@`subq2` `t2`, `t5`)"},
		{"NO_MERGE(t2, t5@subq2)", "NO_MERGE(`t2`, `t5`@`subq2`)"},
		{"NO_MERGE(@subq2 t2, t5)", "NO_MERGE(@`subq2` `t2`, `t5`)"},
		{"MRR(t1)", "MRR(`t1`)"},
		{"MRR(t1@subq1)", "MRR(`t1`@`subq1`)"},
		{"MRR(t1@subq1 idx1, idx2)", "MRR(`t1`@`subq1` `idx1`, `idx2`)"},
		{"MRR(@subq1 t1 idx1, idx2)", "MRR(@`subq1` `t1` `idx1`, `idx2`)"},
		{"NO_MRR(t1)", "NO_MRR(`t1`)"},
		{"NO_MRR(t1@subq1)", "NO_MRR(`t1`@`subq1`)"},
		{"NO_MRR(t1@subq1 idx1, idx2)", "NO_MRR(`t1`@`subq1` `idx1`, `idx2`)"},
		{"NO_MRR(@subq1 t2 idx1, idx2)", "NO_MRR(@`subq1` `t2` `idx1`, `idx2`)"},
		{"NO_ICP(t1)", "NO_ICP(`t1`)"},
		{"NO_ICP(t1@subq1)", "NO_ICP(`t1`@`subq1`)"},
		{"NO_ICP(t1@subq1 idx1, idx2)", "NO_ICP(`t1`@`subq1` `idx1`, `idx2`)"},
		{"NO_ICP(@subq1 t1 idx1, idx2)", "NO_ICP(@`subq1` `t1` `idx1`, `idx2`)"},
		{"NO_RANGE_OPTIMIZATION(t1)", "NO_RANGE_OPTIMIZATION(`t1`)"},
		{"NO_RANGE_OPTIMIZATION(t1@subq1)", "NO_RANGE_OPTIMIZATION(`t1`@`subq1`)"},
		{"NO_RANGE_OPTIMIZATION(t1@subq1 idx1, idx2)", "NO_RANGE_OPTIMIZATION(`t1`@`subq1` `idx1`, `idx2`)"},
		{"NO_RANGE_OPTIMIZATION(@subq1 t1 idx1, idx2)", "NO_RANGE_OPTIMIZATION(@`subq1` `t1` `idx1`, `idx2`)"},
		{"ORDER_INDEX(t1)", "ORDER_INDEX(`t1`)"},
		{"ORDER_INDEX(t1@subq1)", "ORDER_INDEX(`t1`@`subq1`)"},
		{"ORDER_INDEX(t1@subq1 idx1, idx2)", "ORDER_INDEX(`t1`@`subq1` `idx1`, `idx2`)"},
		{"ORDER_INDEX(@subq1 t1 idx1, idx2)", "ORDER_INDEX(@`subq1` `t1` `idx1`, `idx2`)"},
		{"NO_ORDER_INDEX(t1)", "NO_ORDER_INDEX(`t1`)"},
		{"NO_ORDER_INDEX(t1@subq1)", "NO_ORDER_INDEX(`t1`@`subq1`)"},
		{"NO_ORDER_INDEX(t1@subq1 idx1, idx2)", "NO_ORDER_INDEX(`t1`@`subq1` `idx1`, `idx2`)"},
		{"NO_ORDER_INDEX(@subq1 t1 idx1, idx2)", "NO_ORDER_INDEX(@`subq1` `t1` `idx1`, `idx2`)"},
		{"RESOURCE_GROUP(GROUP_NAME)", "RESOURCE_GROUP(GROUP_NAME)"},
		{"SEMIJOIN()", "SEMIJOIN()"},
		{"SEMIJOIN(@subq1)", "SEMIJOIN(@`subq1` )"},
		{"SEMIJOIN(@subq1 MATERIALIZATION, DUPSWEEDOUT)", "SEMIJOIN(@`subq1` MATERIALIZATION, DUPSWEEDOUT)"},
		{"NO_SEMIJOIN()", "NO_SEMIJOIN()"},
		{"NO_SEMIJOIN(@subq1)", "NO_SEMIJOIN(@`subq1` )"},
		{"NO_SEMIJOIN(@subq1 MATERIALIZATION, DUPSWEEDOUT)", "NO_SEMIJOIN(@`subq1` MATERIALIZATION, DUPSWEEDOUT)"},
		{"SKIP_SCAN (t1)", "SKIP_SCAN(`t1`)"},
		{"SKIP_SCAN(t1@subq1)", "SKIP_SCAN(`t1`@`subq1`)"},
		{"SKIP_SCAN(t1@subq1 idx1, idx2)", "SKIP_SCAN(`t1`@`subq1` `idx1`, `idx2`)"},
		{"SKIP_SCAN(@subq1 t1 idx1, idx2)", "SKIP_SCAN(@`subq1` `t1` `idx1`, `idx2`)"},
		{"SUBQUERY()", "SUBQUERY()"},
		{"SUBQUERY(@subq1)", "SUBQUERY(@`subq1` )"},
		{"SUBQUERY(@subq1 INTOEXISTS, MATERIALIZATION)", "SUBQUERY(@`subq1` INTOEXISTS, MATERIALIZATION)"},
	}
	extractNodeFunc := func(node ast.Node) ast.Node {
		hints := node.(*ast.SelectStmt).TableHints
		if len(hints) > 0 {
			return hints[0]
		}
		return &ast.TableOptimizerHint{} // avoid panic
	}
	runNodeRestoreTest(t, testCases, "select /*+ %s */ * from t1 join t2", extractNodeFunc)
}

func TestChangeStmtRestore(t *testing.T) {
	testCases := []NodeRestoreTestCase{
		{"CHANGE PUMP TO NODE_STATE ='paused' FOR NODE_ID '127.0.0.1:9090'", "CHANGE PUMP TO NODE_STATE ='paused' FOR NODE_ID '127.0.0.1:9090'"},
		{"CHANGE DRAINER TO NODE_STATE ='paused' FOR NODE_ID '127.0.0.1:9090'", "CHANGE DRAINER TO NODE_STATE ='paused' FOR NODE_ID '127.0.0.1:9090'"},
	}
	extractNodeFunc := func(node ast.Node) ast.Node {
		return node.(*ast.ChangeStmt)
	}
	runNodeRestoreTest(t, testCases, "%s", extractNodeFunc)
}

func TestBRIESecureText(t *testing.T) {
	testCases := []struct {
		input   string
		secured string
	}{
		{
			input:   "restore database * from 'local:///tmp/br01' snapshot = 23333",
			secured: `^\QRESTORE DATABASE * FROM 'local:///tmp/br01' SNAPSHOT = 23333\E$`,
		},
		{
			input:   "backup database * to 's3://bucket/prefix?region=us-west-2'",
			secured: `^\QBACKUP DATABASE * TO 's3://bucket/prefix?region=us-west-2'\E$`,
		},
		{
			// we need to use regexp to match to avoid the random ordering since a map was used.
			// unfortunately Go's regexp doesn't support lookahead assertion, so the test case below
			// has false positives.
			input:   "backup database * to 's3://bucket/prefix?access-key=abcdefghi&secret-access-key=123&force-path-style=true'",
			secured: `^\QBACKUP DATABASE * TO 's3://bucket/prefix?\E((access-key=xxxxxx|force-path-style=true|secret-access-key=xxxxxx)(&|'$)){3}`,
		},
		{
			input:   "backup database * to 'gcs://bucket/prefix?access-key=irrelevant&credentials-file=/home/user/secrets.txt'",
			secured: `^\QBACKUP DATABASE * TO 'gcs://bucket/prefix?\E((access-key=irrelevant|credentials-file=/home/user/secrets\.txt)(&|'$)){2}`,
		},
	}

	p := parser.New()
	for _, tc := range testCases {
		comment := fmt.Sprintf("input = %s", tc.input)
		node, err := p.ParseOneStmt(tc.input, "", "")
		require.NoError(t, err, comment)
		n, ok := node.(ast.SensitiveStmtNode)
		require.True(t, ok, comment)
		require.Regexp(t, tc.secured, n.SecureText(), comment)
	}
}
