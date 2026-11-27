package snmpx

// 标准 MIB 与华为私有 MIB OID 常量(均不带前导点)。
// 华为私有 OID 在 M0 阶段需以官方 MIB 文档和真机 snmpwalk 结果为准校准。
const (
	// SNMPv2-MIB
	oidSysDescr    = "1.3.6.1.2.1.1.1.0"
	oidSysObjectID = "1.3.6.1.2.1.1.2.0"
	oidSysUpTime   = "1.3.6.1.2.1.1.3.0"
	oidSysName     = "1.3.6.1.2.1.1.5.0"
	oidSysLocation = "1.3.6.1.2.1.1.6.0"

	// IF-MIB
	oidIfDescr       = "1.3.6.1.2.1.2.2.1.2"
	oidIfType        = "1.3.6.1.2.1.2.2.1.3"
	oidIfSpeed       = "1.3.6.1.2.1.2.2.1.5"
	oidIfPhysAddress = "1.3.6.1.2.1.2.2.1.6"
	oidIfAdminStatus = "1.3.6.1.2.1.2.2.1.7"
	oidIfOperStatus  = "1.3.6.1.2.1.2.2.1.8"
	oidIfName        = "1.3.6.1.2.1.31.1.1.1.1"
	oidIfHighSpeed   = "1.3.6.1.2.1.31.1.1.1.15"
	oidIfAlias       = "1.3.6.1.2.1.31.1.1.1.18"

	// IP-MIB (本机管理/业务 IP)
	oidIPAdEntAddr = "1.3.6.1.2.1.4.20.1.1"

	// ARP: 优先 IP-MIB ipNetToPhysicalPhysAddress, 兜底 RFC1213 ipNetToMediaPhysAddress
	oidIPNetToPhysicalPhys = "1.3.6.1.2.1.4.35.1.4"
	oidIPNetToMediaPhys    = "1.3.6.1.2.1.4.22.1.2"

	// IEEE8023-LAG-MIB: dot3adAggPortAttachedAggID (成员口 ifIndex -> 聚合口 ifIndex)
	oidDot3adAggPortAttached = "1.2.840.10006.300.43.1.2.1.1.13"

	// BRIDGE-MIB & Q-BRIDGE-MIB
	oidDot1dBaseBridgeAddr   = "1.3.6.1.2.1.17.1.1.0"
	oidDot1dBasePortIfIndex  = "1.3.6.1.2.1.17.1.4.1.2"
	oidDot1dTpFdbPort        = "1.3.6.1.2.1.17.4.3.1.2"
	oidDot1dTpFdbStatus      = "1.3.6.1.2.1.17.4.3.1.3"
	oidDot1qTpFdbPort        = "1.3.6.1.2.1.17.7.1.2.2.1.2"
	oidDot1qTpFdbStatus      = "1.3.6.1.2.1.17.7.1.2.2.1.3"

	// LLDP-MIB (IEEE 802.1AB)
	oidLldpLocChassisID    = "1.0.8802.1.1.2.1.3.2.0"
	oidLldpLocPortID       = "1.0.8802.1.1.2.1.3.7.1.3"
	oidLldpLocPortDesc     = "1.0.8802.1.1.2.1.3.7.1.4"
	oidLldpRemChassisSub   = "1.0.8802.1.1.2.1.4.1.1.4"
	oidLldpRemChassisID    = "1.0.8802.1.1.2.1.4.1.1.5"
	oidLldpRemPortSub      = "1.0.8802.1.1.2.1.4.1.1.6"
	oidLldpRemPortID       = "1.0.8802.1.1.2.1.4.1.1.7"
	oidLldpRemPortDesc     = "1.0.8802.1.1.2.1.4.1.1.8"
	oidLldpRemSysName      = "1.0.8802.1.1.2.1.4.1.1.9"
	oidLldpRemSysDesc      = "1.0.8802.1.1.2.1.4.1.1.10"
	oidLldpRemSysCapEn     = "1.0.8802.1.1.2.1.4.1.1.12"
	oidLldpRemManAddrIfSub = "1.0.8802.1.1.2.1.4.2.1.3"

	// ENTITY-MIB (物理实体: 堆叠成员数 / 序列号)
	oidEntPhysicalClass     = "1.3.6.1.2.1.47.1.1.1.1.5"
	oidEntPhysicalSerialNum = "1.3.6.1.2.1.47.1.1.1.1.11"

	// HUAWEI-ENTITY-EXTENT-MIB (企业号 2011; 需真机校准)
	oidHwEntityCPUUsage    = "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.5"
	oidHwEntityMemUsage    = "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.7"
	oidHwEntityTemperature = "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.11"
)
