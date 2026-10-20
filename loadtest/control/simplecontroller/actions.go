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
	"strings"
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

// createDirectPost sends a message to a deterministic one-to-one Direct
// Channel. Test users are paired by the numeric suffix in their username:
// 1<->2, 3<->4, 5<->6. The Direct Channel is resolved once per controller and
// then cached, so channel creation is not included in the steady-state message
// traffic.
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

func groupUserNumber(username string) (int, bool) {
	if len(username) < 5 {
		return 0, false
	}

	suffix := username[len(username)-5:]
	n, err := strconv.Atoi(suffix)
	return n, err == nil
}

func groupUserPrefix(username string) string {
	if len(username) < 5 {
		return ""
	}

	return username[:len(username)-5]
}

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
	groupStart := ((userNo-1)/groupSize)*groupSize + 1

	otherUserIds := make([]string, 0, groupSize-1)
	for i := 0; i < groupSize; i++ {
		memberNo := groupStart + i
		memberUsername := fmt.Sprintf("%s%05d", prefix, memberNo)

		if memberUsername == username {
			continue
		}

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

func (c *SimpleController) createDirectPost(u user.User) control.UserActionResponse {
	if c.directChannelID == "" {
		username := u.Store().Username()
		suffix := user.TestUserSuffixRegexp.FindString(username)
		if suffix == "" {
			return control.UserActionResponse{
				Err: control.NewUserError(
					fmt.Errorf("direct message pairing: username %q has no numeric suffix", username),
				),
			}
		}

		userNumber, err := strconv.Atoi(suffix)
		if err != nil || userNumber < 1 {
			return control.UserActionResponse{
				Err: control.NewUserError(
					fmt.Errorf("direct message pairing: invalid suffix %q in username %q", suffix, username),
				),
			}
		}

		partnerNumber := userNumber + 1
		if userNumber%2 == 0 {
			partnerNumber = userNumber - 1
		}

		prefix := strings.TrimSuffix(username, suffix)
		partnerUsername := fmt.Sprintf(
			"%s%0*d",
			prefix,
			len(suffix),
			partnerNumber,
		)

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

// createChannelGroupPost is for S03B channel group performance testing.
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

	if !strings.Contains(post.Message, "send_unix_nano=") {
		return
	}

	var sendUnixNano int64
	for _, part := range strings.Fields(post.Message) {
		if strings.HasPrefix(part, "send_unix_nano=") {
			value := strings.TrimPrefix(part, "send_unix_nano=")
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				sendUnixNano = parsed
			}
			break
		}
	}

	if sendUnixNano <= 0 {
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
