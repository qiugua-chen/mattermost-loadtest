# SimpleController 配置


## Actions

*[]ActionDefinition*

SimpleController 将循环执行的用户动作定义列表。

### ActionDefinition

#### ActionId

*string*

动作名称，映射到 SimpleController 内部注册的动作。可用动作清单见[这里](https://github.com/mattermost/mattermost-load-test-ng/blob/master/loadtest/control/simplecontroller/controller.go#L137)。

> **扩展版说明**：本扩展版新增三个动作，同样按名引用：
> `CreateDirectPost`（S02 单聊）、`CreateGroupPost`（S03 群聊）、
> `CreateChannelGroupPost`（S03B 频道群发）。

#### WaitAfterMs

*int*

动作执行完成后的等待时间（毫秒）。配合 RunPeriod 控制动作节奏。

#### RunPeriod

*int*

动作的执行周期权重。值越大，动作被执行的概率越低。

- RunPeriod > 0：动作参与执行。
- RunPeriod == 0：动作被跳过。
- RunPeriod < 0：配置校验时报错。
