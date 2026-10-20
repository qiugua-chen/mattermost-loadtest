# GenController 配置

> 使用背景见[数据生成](../generating_data.md)。

## NumTeams

*int64*

要创建的团队数量。

## NumChannelsDM

*int64*

要创建的 DM（私信）频道数量。

## NumChannelsGM

*int64*

要创建的 GM（群聊）频道数量。

## NumChannelsPrivate

*int64*

要创建的私有频道数量。

## NumChannelsPublic

*int64*

要创建的公开频道数量。

## NumCPAFields

*int64*

要创建的自定义用户属性（Custom Profile Attribute）字段数量。

## NumPosts

*int64*

要创建的帖子数量。

## NumReactions

*int64*

要创建的表情回应（reaction）数量。

## NumPostReminders

*int64*

要创建的帖子提醒数量。

## NumSidebarCategories

*int64*

要创建的侧边栏分类数量。

## NumFollowedThreads

*int64*

要关注的讨论串（thread）数量。

## PercentReplies

*float64*

回复帖占总帖子数的百分比。

## PercentRepliesInLongThreads

*float64*

长讨论串中的回复帖占总回复帖数的百分比。

## PercentUrgentPosts

*float64*

标记为紧急优先级（urgent）的帖子占总帖子数的百分比。

## ChannelMembersDistribution

*[]gencontroller.ChannelMemberDistribution*

按频道成员上限划分的频道成员分布。

### MemberLimit

本组频道允许的最大成员数。0 表示不限。

### PercentChannels

采用上述成员上限的频道所占百分比。

### Probability

该组频道被选中的频率权重。

#### 说明

各组频道百分比之和必须等于 1。
