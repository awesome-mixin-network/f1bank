# f1bank

### 交互流程
- 用户查询F1Bank持仓，并根据持仓可以计算出相应的每个币的额度
- 用户向F1Bank申请某个币的快速入金，包括币种，金额
- 系统判断是否受理该快速入金，如果接受，则给用户返回一个broker，并对相应的币进行锁仓
- 用户在指定时间内将币转到相应的broker，则系统将相应的币转给相应的用户
- 如果系统超过半小时没有进行转账，则取消锁仓。假如在一天内收到该笔转账，如果系统余额充足，则第一时间给用户转账；否则等该币转账在mixin体系内入金成功后再做转账
- 如果一天后还没有收到转账，则取消订单
- TODO: 用户信用体系; 系统持仓不足时，可允许一些用户的存储行为，并付出相应利息。

### 鉴权相关
- Mixin Messenger 用户可以直接通过dapp进行操作
- 第三方用户，如fox用户等，可以给dapp转一笔转账，并在转账中附带一个公钥。后续的交互可以通过相应私钥进行鉴权