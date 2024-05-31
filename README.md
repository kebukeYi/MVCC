# MVCC

理解MVCC

## 可重复读隔离

1. row_trx_id 小于 min_trx_id 可读； 之前的事务就已经将该条数据修改了并提交了事务；
2. row_trx_id **大于等于** max_trx_id+1 不可读； 新的事务修改了这行数据的值并提交了事务；
3. **( min_trx_id，... ，max_trx_id )** 在活跃列表区间

- row_trx_id 在 m_ids 数组中，不可读；
- row_trx_id 不在 m_ids 数组中，可读； **读提交的事务隔离级别下，会出现这种现象**；

4. m_ids[ ]数组中保存的是 未提交的事务id，只要提交，就会从数组中剔除； 注意：就算事务不提交，数据同样会产生undolog链；
5. 凡是在当前事务T1开启，在未提交期间，后面开启的新事务T2,T3,T4...等等全都是不可读T1的修改的值，全都是跟T1事务绑定；因此不管T1事务是否提交，绑定一起的事务都无法读取到其T1事务修改的值； T3，T4和T2的关系类似；

## 读已提交隔离

1. 凡是数据的事务id没在 全局的活跃列表中，都可读；因为当有事务提交后，其事务id会从全局活跃列表中剔除；不提交，不剔除，也就不能读 未提交的数据；



### 参考

[Rust 练手项目—实现 MVCC 多版本并发控制](https://mp.weixin.qq.com/s?__biz=MzI0Njg1MTUxOA%3D%3D&mid=2247486521&idx=1&sn=d889e056101435f55c7d85fc3fb797ce)
[MVCC的底层原理](https://blog.csdn.net/weixin_47184173/article/details/117433478)

[一文搞懂undo log版本链与ReadView机制如何让事务读取到该读的数据](https://mp.weixin.qq.com/s/OsSQiCR6076bdoUjZKPXPg)

[在 MySQL 中是如何通过 MVCC 机制来解决不可重复读和幻读问题的？](https://mp.weixin.qq.com/s/Mq9UcV94mTxi6J5SwthV7g)

[在读提交的事务隔离级别下，MVCC 机制是如何工作的？](https://mp.weixin.qq.com/s/QCyzs91AavUD0o23Y4wLjw)