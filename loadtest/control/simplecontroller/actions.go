// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package simplecontroller

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/mattermost/mattermost-load-test-ng/loadtest/control"
	"github.com/mattermost/mattermost-load-test-ng/loadtest/store"
	"github.com/mattermost/mattermost-load-test-ng/loadtest/user"
	"github.com/mattermost/mattermost/server/public/model"
)

type UserAction struct {
	run       control.UserAction
	waitAfter time.Duration
	runPeriod int
}

func (c *SimpleController) sendDirectMessage(userID string) control.UserStatus {
	channelId := model.GetDMNameFromIds(userID, c.user.Store().Id())
	ok, err := c.user.Store().Channel(channelId)
	if err != nil {
		return c.newErrorStatus(err)
	}
	// We check if a direct channel has been made between the users,
	// and send the message only if it exists.
	if ok == nil {
		return c.newInfoStatus("skipping sending direct message")
	}

	postId, err := c.user.CreatePost(&model.Post{
		Message:   "Lorem ipsum dolor sit amet, consectetur adipiscing elit",
		ChannelId: channelId,
		CreateAt:  time.Now().Unix() * 1000,
	})
	if err != nil {
		return c.newErrorStatus(err)
	}

	return c.newInfoStatus(fmt.Sprintf("direct post created, id %v", postId))
}

func (c *SimpleController) scrollChannel(u user.User) control.UserActionResponse {
	collapsedThreads, resp := control.CollapsedThreadsEnabled(u)
	if resp.Err != nil {
		return resp
	}

	team, err := c.user.Store().RandomTeam(store.SelectMemberOf)
	if err != nil {
		return control.UserActionResponse{Err: control.NewUserError(err)}
	}
	channel, err := c.user.Store().RandomChannel(team.Id, store.SelectMemberOf)
	if err != nil {
		return control.UserActionResponse{Err: control.NewUserError(err)}
	}

	err = c.user.GetPostsForChannel(channel.Id, 0, 1, collapsedThreads)
	if err != nil {
		return control.UserActionResponse{Err: control.NewUserError(err)}
	}
	posts, err := c.user.Store().ChannelPostsSorted(channel.Id, true)
	if err != nil {
		return control.UserActionResponse{Err: control.NewUserError(err)}
	}

	if len(posts) == 0 {
		return control.UserActionResponse{Info: fmt.Sprintf("no posts in channel %v", channel.Id)}
	}

	postId := posts[0].Id // get the oldest post
	const NUM_OF_SCROLLS = 3
	const SLEEP_BETWEEN_SCROLL = 1000
	for i := 0; i < NUM_OF_SCROLLS; i++ {
		if _, err = c.user.GetPostsBefore(channel.Id, postId, 0, 10, collapsedThreads); err != nil {
			return control.UserActionResponse{Err: control.NewUserError(err)}
		}
		posts, err := c.user.Store().ChannelPostsSorted(channel.Id, false)
		if err != nil {
			return control.UserActionResponse{Err: control.NewUserError(err)}
		}
		postId = posts[0].Id // get the newest post
		idleTime := time.Duration(math.Round(float64(SLEEP_BETWEEN_SCROLL) * c.rate))
		time.Sleep(time.Millisecond * idleTime)
	}
	return control.UserActionResponse{Info: fmt.Sprintf("scrolled channel %v %d times", channel.Id, NUM_OF_SCROLLS)}
}

func (c *SimpleController) updateProfile(u user.User) control.UserActionResponse {
	userId := c.user.Store().Id()

	userName := control.RandomizeUserName(c.user.Store().Username())
	nickName := fmt.Sprintf("testNickName%d", c.id)
	firstName := fmt.Sprintf("firstName%d", c.id)
	lastName := fmt.Sprintf("lastName%d", c.id)
	err := c.user.PatchUser(userId, &model.UserPatch{
		Username:  &userName,
		Nickname:  &nickName,
		FirstName: &firstName,
		LastName:  &lastName,
	})
	if err != nil {
		return control.UserActionResponse{Err: control.NewUserError(err)}
	}

	return control.UserActionResponse{Info: "user patched"}
}

func (c *SimpleController) updateTeam(user.User) control.UserActionResponse {
	if ok, err := c.user.IsTeamAdmin(); err != nil {
		return control.UserActionResponse{Err: control.NewUserError(err)}
	} else if !ok {
		return control.UserActionResponse{Info: "user doesn't have permission to update"}
	}

	team, err := c.user.Store().RandomTeam(store.SelectMemberOf)
	if err != nil {
		return control.UserActionResponse{Err: control.NewUserError(err)}
	}
	team.DisplayName = control.RandomizeTeamDisplayName(team.DisplayName)

	if err := c.user.UpdateTeam(&team); err != nil {
		return control.UserActionResponse{Err: control.NewUserError(err)}
	}

	return control.UserActionResponse{Info: "team updated"}
}

// reload performs all actions done when a user reloads the browser.
// If full parameter is enabled, it also disconnects and reconnects
// the WebSocket connection.
func (c *SimpleController) reload(full bool) control.UserActionResponse {
	if full {
		err := c.disconnect()
		if err != nil {
			return control.UserActionResponse{Err: control.NewUserError(err)}
		}
		c.user.ClearUserData()

		if err := c.connect(); err != nil {
			return control.UserActionResponse{Err: control.NewUserError(err)}
		}
	}

	return control.Reload(c.user)
}

func (c *SimpleController) connect() error {
	if !atomic.CompareAndSwapInt32(&c.connectedFlag, 0, 1) {
		return errors.New("already connected")
	}
	errChan, err := c.user.Connect()
	if err != nil {
		atomic.StoreInt32(&c.connectedFlag, 0)
		return fmt.Errorf("connect failed %w", err)
	}

	c.wg.Add(2)
	go func() {
		defer c.wg.Done()
		for err := range errChan {
			c.status <- c.newErrorStatus(err)
		}
	}()
	go c.wsEventHandler(c.wg)
	return nil
}

func (c *SimpleController) disconnect() error {
	if !atomic.CompareAndSwapInt32(&c.connectedFlag, 1, 0) {
		return errors.New("not connected")
	}

	err := c.user.Disconnect()
	if err != nil {
		return fmt.Errorf("disconnect failed %w", err)
	}
	c.wg.Wait()
	return nil
}

// groupEnvInt 读取整型环境变量（群聊场景参数用），未设置、解析失败或 <=0 时返回默认值。
func groupEnvInt(name string, defaultValue int) int {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return defaultValue
	}

	return parsed
}

// groupUserNumber 从用户名末尾解析 5 位数字编号（如 ltuser00042 -> 42）。
// 约定：压测账号命名格式为 <前缀><5位数字>，编号用于确定性分组/配对。
func groupUserNumber(username string) (int, bool) {
	if len(username) < 5 {
		return 0, false
	}

	suffix := username[len(username)-5:]
	n, err := strconv.Atoi(suffix)
	return n, err == nil
}

// groupUserPrefix 取用户名去掉末尾 5 位数字后的前缀部分（如 ltuser00042 -> ltuser）。
func groupUserPrefix(username string) string {
	if len(username) < 5 {
		return ""
	}

	return username[:len(username)-5]
}

// createGroupPost 群聊发消息动作（场景 S03_GM）。
// 按用户名数字编号确定性分组：编号 1 起每 LT_GROUP_SIZE 人一组（默认 3 人），
// 组内成员两两确定，无需预先建群——压测时由首发用户创建 Group Channel，
// Mattermost 对相同成员组合会复用已有频道，天然幂等。
// 消息体内嵌 send_unix_nano 发送时间戳与唯一 msg id，供接收端
// recordReceiverLatency 计算端到端延迟。
func (c *SimpleController) createGroupPost(u user.User) control.UserActionResponse {
	groupSize := groupEnvInt("LT_GROUP_SIZE", 3)
	if groupSize < 3 {
		groupSize = 3
	}

	username := u.Store().Username()
	userNo, ok := groupUserNumber(username)
	if !ok {
		return control.UserActionResponse{Err: control.NewUserError(fmt.Errorf("cannot parse user number from username %q", username))}
	}

	prefix := groupUserPrefix(username)
	if prefix == "" {
		return control.UserActionResponse{Err: control.NewUserError(fmt.Errorf("cannot parse user prefix from username %q", username))}
	}

	// 用户编号按 1 开始分组：
	// 3人群：00001/00002/00003，00004/00005/00006 ...
	otherUserIds := make([]string, 0, groupSize-1)
	for _, memberNo := range groupMemberNumbers(userNo, groupSize) {
		if memberNo == userNo {
			continue
		}
		memberUsername := fmt.Sprintf("%s%05d", prefix, memberNo)

		members, err := u.GetUsersByUsernames([]string{memberUsername})
		if err != nil {
			return control.UserActionResponse{Err: control.NewUserError(fmt.Errorf("lookup group member %q failed: %w", memberUsername, err))}
		}
		if len(members) == 0 || members[0] == "" {
			return control.UserActionResponse{Err: control.NewUserError(fmt.Errorf("lookup group member %q failed: empty user", memberUsername))}
		}

		otherUserIds = append(otherUserIds, members[0])
	}

	if len(otherUserIds) != groupSize-1 {
		return control.UserActionResponse{Err: control.NewUserError(fmt.Errorf("not enough group members, group_size=%d got_other_members=%d", groupSize, len(otherUserIds)))}
	}

	channelId, err := u.CreateGroupChannel(otherUserIds)
	if err != nil {
		return control.UserActionResponse{Err: control.NewUserError(fmt.Errorf("create group channel failed: %w", err))}
	}

	sendUnixNano := time.Now().UnixNano()
	messageID := model.NewId()
	postId, err := u.CreatePost(&model.Post{
		Message: fmt.Sprintf(
			"lt-message-test scenario=S03_GM controller=%d group_size=%d send_unix_nano=%d msg=%s",
			c.id,
			groupSize,
			sendUnixNano,
			messageID,
		),
		ChannelId: channelId,
		CreateAt:  time.Now().Unix() * 1000,
	})
	if err != nil {
		return control.UserActionResponse{Err: control.NewUserError(fmt.Errorf("create group post in channel %q failed: %w", channelId, err))}
	}

	return control.UserActionResponse{Info: fmt.Sprintf("group post created, group_size=%d channel_id=%s post_id=%s", groupSize, channelId, postId)}
}

// createDirectPost 单聊（私信）发消息动作（场景 S02_DIRECT）。
// 配对规则：按用户名数字后缀确定性配对 1<->2、3<->4、5<->6……，
// 即奇数编号发给 +1，偶数编号发给 -1，无需人工维护配对表。
// DM 频道首次解析后缓存在 c.directChannelID，稳态压测流量中不含建频道开销。
// 消息体内嵌 send_unix_nano 供端到端延迟计算。
func (c *SimpleController) createDirectPost(u user.User) control.UserActionResponse {
	if c.directChannelID == "" {
		partnerUsername, err := directPartnerUsername(u.Store().Username())
		if err != nil {
			return control.UserActionResponse{Err: control.NewUserError(err)}
		}

		partnerIDs, err := u.GetUsersByUsernames([]string{partnerUsername})
		if err != nil {
			return control.UserActionResponse{
				Err: control.NewUserError(
					fmt.Errorf("direct message pairing: lookup %q failed: %w", partnerUsername, err),
				),
			}
		}

		if len(partnerIDs) != 1 || partnerIDs[0] == "" {
			return control.UserActionResponse{
				Err: control.NewUserError(
					fmt.Errorf(
						"direct message pairing: expected one user for %q, got %d",
						partnerUsername,
						len(partnerIDs),
					),
				),
			}
		}

		channelID, err := u.CreateDirectChannel(partnerIDs[0])
		if err != nil {
			return control.UserActionResponse{
				Err: control.NewUserError(
					fmt.Errorf("create direct channel with %q failed: %w", partnerUsername, err),
				),
			}
		}

		if err := u.GetChannel(channelID); err != nil {
			return control.UserActionResponse{
				Err: control.NewUserError(
					fmt.Errorf("load direct channel %q failed: %w", channelID, err),
				),
			}
		}

		channel, err := u.Store().Channel(channelID)
		if err != nil {
			return control.UserActionResponse{
				Err: control.NewUserError(
					fmt.Errorf("read direct channel %q from store failed: %w", channelID, err),
				),
			}
		}

		if channel.Type != model.ChannelTypeDirect {
			return control.UserActionResponse{
				Err: control.NewUserError(
					fmt.Errorf(
						"channel %q has type %q, expected direct channel",
						channelID,
						channel.Type,
					),
				),
			}
		}

		if err := u.SetCurrentChannel(channel); err != nil {
			return control.UserActionResponse{
				Err: control.NewUserError(
					fmt.Errorf("set direct channel %q current failed: %w", channelID, err),
				),
			}
		}

		if err := u.UpdateActiveChannel(channelID); err != nil {
			return control.UserActionResponse{
				Err: control.NewUserError(
					fmt.Errorf("update active direct channel %q failed: %w", channelID, err),
				),
			}
		}

		c.directChannelID = channelID
	}

	sendUnixNano := time.Now().UnixNano()
	messageID := model.NewId()
	message := fmt.Sprintf(
		"lt-message-test scenario=S02_DIRECT sender=%s send_unix_nano=%d msg=%s",
		u.Store().Username(),
		sendUnixNano,
		messageID,
	)

	postID, err := u.CreatePost(&model.Post{
		ChannelId: c.directChannelID,
		Message:   message,
	})
	if err != nil {
		return control.UserActionResponse{
			Err: control.NewUserError(
				fmt.Errorf("create direct post in channel %q failed: %w", c.directChannelID, err),
			),
		}
	}

	return control.UserActionResponse{
		Info: fmt.Sprintf(
			"direct post created, channel_id=%s post_id=%s",
			c.directChannelID,
			postID,
		),
	}
}

// createChannelGroupPost 频道群发动作（场景 S03_CHANNEL_GROUP）：
// 单一频道内"N 发 M 收"模型——全部用户先入群并在线待命（接收方），
// 仅前 LT_CHANNEL_GROUP_SENDERS 个 controller 发消息，考核频道广播扇出能力。
//
// 门控机制：LT_ACTION_START_FILE 指向的信号文件不存在时，动作只做
// "确保已加入频道"的就绪初始化并返回；信号文件出现后才真正开始发送。
// 配合 loadtest-stage-functions.sh 实现"全员就绪后再开压"的同步起跑。
//
// 必需环境变量：LT_CHANNEL_GROUP_CHANNEL_ID（目标频道 ID）。
func (c *SimpleController) createChannelGroupPost(u user.User) control.UserActionResponse {
	channelID := os.Getenv("LT_CHANNEL_GROUP_CHANNEL_ID")
	if channelID == "" {
		return control.UserActionResponse{Err: fmt.Errorf("LT_CHANNEL_GROUP_CHANNEL_ID is empty")}
	}

	startFile := os.Getenv("LT_ACTION_START_FILE")
	formalStarted := false
	if startFile != "" {
		if _, err := os.Stat(startFile); err == nil {
			formalStarted = true
		}
	}

	if !formalStarted {
		userID := u.Store().Id()
		if userID == "" {
			return control.UserActionResponse{Err: fmt.Errorf("empty current user id")}
		}

		cm, err := u.Store().ChannelMember(channelID, userID)
		if err != nil {
			return control.UserActionResponse{Err: control.NewUserError(err)}
		}

		if cm.UserId == "" {
			if err := u.AddChannelMember(channelID, userID); err != nil {
				return control.UserActionResponse{Err: control.NewUserError(err)}
			}
		}

		return control.UserActionResponse{Info: fmt.Sprintf("channel group initialized, channel=%s", channelID)}
	}

	senders := 1
	if v := os.Getenv("LT_CHANNEL_GROUP_SENDERS"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			senders = n
		}
	}

	if c.id >= senders {
		return control.UserActionResponse{Info: "channel group receiver online"}
	}

	sendUnixNano := time.Now().UnixNano()
	messageID := model.NewId()

	postID, err := u.CreatePost(&model.Post{
		ChannelId: channelID,
		Message: fmt.Sprintf(
			"lt-message-test scenario=S03_CHANNEL_GROUP controller=%d send_unix_nano=%d msg=%s",
			c.id,
			sendUnixNano,
			messageID,
		),
	})
	if err != nil {
		return control.UserActionResponse{Err: control.NewUserError(err)}
	}

	return control.UserActionResponse{Info: fmt.Sprintf("channel group post created, id=%s", postID)}
}

// recordReceiverLatency 端到端接收延迟采集（接收侧）。
// 在 WS posted 事件回调中实时解析消息体内嵌的 send_unix_nano 发送时间戳，
// 计算 发送->接收 全链路延迟（含服务端处理 + WS 推送），
// 以 CSV 行追加写入 LT_RECEIVER_LATENCY_FILE 指定的文件，
// 供 scripts/summarize_receiver_latency.py 做分位数统计（P50/P95/P99）。
// 非本工具发出的消息（无 send_unix_nano 标记）直接忽略。
func (c *SimpleController) recordReceiverLatency(event *model.WebSocketEvent) {
	if event == nil || event.EventType() != model.WebsocketEventPosted {
		return
	}

	raw := event.GetData()["post"]
	if raw == nil {
		return
	}

	post := &model.Post{}
	switch v := raw.(type) {
	case string:
		if v == "" {
			return
		}
		if err := json.Unmarshal([]byte(v), post); err != nil {
			return
		}
	case map[string]interface{}:
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		if err := json.Unmarshal(b, post); err != nil {
			return
		}
	default:
		return
	}

	sendUnixNano, ok := parseSendUnixNano(post.Message)
	if !ok {
		return
	}

	output := os.Getenv("LT_RECEIVER_LATENCY_FILE")
	if output == "" {
		return
	}

	latencyMs := float64(time.Now().UnixNano()-sendUnixNano) / 1e6

	f, err := os.OpenFile(output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	if stat, err := f.Stat(); err == nil && stat.Size() == 0 {
		fmt.Fprintln(f, "epoch,controller_id,channel_id,post_id,latency_ms")
	}

	fmt.Fprintf(f, "%d,%d,%s,%s,%.3f\n", time.Now().Unix(), c.id, post.ChannelId, post.Id, latencyMs)
}
