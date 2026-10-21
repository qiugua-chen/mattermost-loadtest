// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package simplecontroller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-load-test-ng/loadtest/user"
)

// 本文件为本扩展版（二开）新增的纯函数助手，从各消息场景动作中提取，
// 不依赖网络/服务端，可独立单测（见 ext_helpers_test.go）。
// 约定：压测账号命名格式为 <前缀><数字编号>，编号是确定性配对/分组的唯一依据。

// groupMemberNumbers 计算编号 selfNo 所在的确定性分组全体成员编号（含自身）。
// 规则：编号从 1 起，每 groupSize 个连续编号为一组（1~size、size+1~2*size ……）。
// 参数非法（selfNo<1 或 groupSize<1）时返回 nil。
func groupMemberNumbers(selfNo, groupSize int) []int {
	if selfNo < 1 || groupSize < 1 {
		return nil
	}
	start := ((selfNo-1)/groupSize)*groupSize + 1
	members := make([]int, 0, groupSize)
	for i := 0; i < groupSize; i++ {
		members = append(members, start+i)
	}
	return members
}

// directPartnerUsername 计算单聊确定性配对对象的用户名（场景 S02_DIRECT）。
// 规则：用户名尾部数字编号 奇数配对 +1、偶数配对 -1（1<->2、3<->4 ……），
// 配对用户名保持与原用户名相同的前缀和数字位宽。
// 无数字后缀或编号 <1 时返回错误。
func directPartnerUsername(username string) (string, error) {
	suffix := user.TestUserSuffixRegexp.FindString(username)
	if suffix == "" {
		return "", fmt.Errorf("direct message pairing: username %q has no numeric suffix", username)
	}

	userNumber, err := strconv.Atoi(suffix)
	if err != nil || userNumber < 1 {
		return "", fmt.Errorf("direct message pairing: invalid suffix %q in username %q", suffix, username)
	}

	partnerNumber := userNumber + 1
	if userNumber%2 == 0 {
		partnerNumber = userNumber - 1
	}

	prefix := strings.TrimSuffix(username, suffix)
	return fmt.Sprintf("%s%0*d", prefix, len(suffix), partnerNumber), nil
}

// parseSendUnixNano 从消息体解析本工具内嵌的 send_unix_nano 发送时间戳，
// 供 recordReceiverLatency 计算端到端延迟。
// 无标记、数值非法或 <=0 时返回 ok=false（非本工具消息/损坏消息直接忽略）。
func parseSendUnixNano(message string) (sendUnixNano int64, ok bool) {
	if !strings.Contains(message, "send_unix_nano=") {
		return 0, false
	}
	for _, part := range strings.Fields(message) {
		if strings.HasPrefix(part, "send_unix_nano=") {
			parsed, err := strconv.ParseInt(strings.TrimPrefix(part, "send_unix_nano="), 10, 64)
			if err != nil || parsed <= 0 {
				return 0, false
			}
			return parsed, true
		}
	}
	return 0, false
}
