// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package simplecontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本扩展版二开核心逻辑的单元测试：
// 覆盖 确定性配对（S02）、确定性分组（S03A）、延迟采集标记解析 三类纯函数，
// 不依赖服务端，可在 CI 中直接运行。

func TestGroupUserNumber(t *testing.T) {
	cases := []struct {
		username string
		want     int
		ok       bool
	}{
		{"imlt00001", 1, true},
		{"ltuser00042", 42, true},
		{"imlts0112000", 12000, true},
		{"00007", 7, true}, // 纯数字用户名也允许
		{"abc", 0, false},  // 长度不足 5
		{"user123", 0, false},
		{"user1234x", 0, false}, // 末 5 位非纯数字
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := groupUserNumber(c.username)
		assert.Equal(t, c.ok, ok, "username=%q", c.username)
		if c.ok {
			assert.Equal(t, c.want, got, "username=%q", c.username)
		}
	}
}

func TestGroupUserPrefix(t *testing.T) {
	assert.Equal(t, "imlt", groupUserPrefix("imlt00001"))
	assert.Equal(t, "ltuser", groupUserPrefix("ltuser00042"))
	assert.Equal(t, "", groupUserPrefix("00007"))
	assert.Equal(t, "", groupUserPrefix("abc"))
}

func TestGroupEnvInt(t *testing.T) {
	const key = "LT_TEST_GROUP_ENV_INT"

	// 未设置 -> 默认值
	assert.Equal(t, 3, groupEnvInt(key, 3))

	t.Setenv(key, "5")
	assert.Equal(t, 5, groupEnvInt(key, 3))

	// 0 / 负数 / 非数字 -> 默认值
	for _, bad := range []string{"0", "-2", "abc", "3.5"} {
		t.Setenv(key, bad)
		assert.Equal(t, 3, groupEnvInt(key, 3), "value=%q", bad)
	}
}

func TestGroupMemberNumbers(t *testing.T) {
	cases := []struct {
		selfNo   int
		size     int
		expected []int
	}{
		{1, 3, []int{1, 2, 3}},
		{2, 3, []int{1, 2, 3}},
		{3, 3, []int{1, 2, 3}},
		{4, 3, []int{4, 5, 6}},
		{5, 4, []int{5, 6, 7, 8}},
		{8, 4, []int{5, 6, 7, 8}},
		{1, 1, []int{1}}, // 单人组（实际场景 groupSize 最小为 3，这里只验证公式）
		// 非法输入
		{0, 3, nil},
		{1, 0, nil},
		{-1, 3, nil},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, groupMemberNumbers(c.selfNo, c.size),
			"selfNo=%d size=%d", c.selfNo, c.size)
	}
}

func TestDirectPartnerUsername(t *testing.T) {
	cases := []struct {
		username string
		want     string
	}{
		{"imlt00001", "imlt00002"}, // 奇数 -> +1
		{"imlt00002", "imlt00001"}, // 偶数 -> -1
		{"imlt00009", "imlt00010"},
		{"imlt00010", "imlt00009"},
		{"ltuser1", "ltuser2"}, // 位宽不固定时按原后缀宽度补齐
		{"imlts0111999", "imlts0112000"},
	}
	for _, c := range cases {
		got, err := directPartnerUsername(c.username)
		require.NoError(t, err, "username=%q", c.username)
		assert.Equal(t, c.want, got, "username=%q", c.username)

		// 配对关系必须对称（互为对象）
		back, err := directPartnerUsername(got)
		require.NoError(t, err)
		assert.Equal(t, c.username, back, "pairing not symmetric for %q", c.username)
	}
}

func TestDirectPartnerUsernameErrors(t *testing.T) {
	for _, username := range []string{"nodigits", "", "user00000"} {
		_, err := directPartnerUsername(username)
		assert.Error(t, err, "username=%q should fail", username)
	}
}

func TestParseSendUnixNano(t *testing.T) {
	cases := []struct {
		message string
		want    int64
		ok      bool
	}{
		{"lt-message-test scenario=S02_DIRECT controller=3 send_unix_nano=1728000000123456789 msg=abc", 1728000000123456789, true},
		{"lt-message-test scenario=S03_GM group_size=3 send_unix_nano=1 msg=x", 1, true},
		{"send_unix_nano=1728000000123456789", 1728000000123456789, true},
		// 无标记 / 非法值 / 非正数 -> 忽略
		{"hello world", 0, false},
		{"", 0, false},
		{"lt-message-test send_unix_nano=abc msg=x", 0, false},
		{"lt-message-test send_unix_nano=0 msg=x", 0, false},
		{"lt-message-test send_unix_nano=-5 msg=x", 0, false},
		// 备注字段中含相似字符串但无独立标记字段 -> 忽略
		{"lt-message-test msg=send_unix_nano=123", 0, false},
	}
	for _, c := range cases {
		got, ok := parseSendUnixNano(c.message)
		assert.Equal(t, c.ok, ok, "message=%q", c.message)
		if c.ok {
			assert.Equal(t, c.want, got, "message=%q", c.message)
		}
	}
}
