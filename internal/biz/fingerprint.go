package biz

import (
	"regexp"
	"strings"
)

// Fingerprint 设备指纹识别结果。
type Fingerprint struct {
	Vendor    string
	Family    string
	Model     string
	OSVersion string
}

// 企业号 → 厂商
var enterpriseVendors = map[string]string{
	"2011":  "Huawei",
	"25506": "H3C",
	"4881":  "Ruijie",
	"9":     "Cisco",
	"2636":  "Juniper",
	"14988": "MikroTik",
	"3902":  "ZTE",
	"1916":  "Extreme",
	"11":    "HP",
	"674":   "Dell",
	"12356": "Fortinet",
	"25461": "PaloAlto",
	"30065": "Arista",
	"14823": "Aruba",
	"35047": "Sangfor",
	"11863": "TP-Link",
	"171":   "D-Link",
	"5651":  "Maipu",
	"3320":  "BDCOM",
}

// 通用 SNMP agent 企业号(如 Net-SNMP/UCD)，不能直接代表硬件厂商，需结合 sysDescr 识别。
var genericEnterpriseIDs = map[string]bool{
	"8072": true, // Net-SNMP
	"2021": true, // UCD-SNMP
}

var (
	// Huawei: Huawei Versatile Routing Platform Software VRP (R) software, Version 5.170 (S5700-28C-HI V200R011C10SPC600)
	huaweiModelVer = regexp.MustCompile(`\(([A-Za-z][A-Za-z0-9\-/]*)\s+(V\d{3}R\d{3}C\d{2}[A-Za-z0-9]*)\)`)
	huaweiModelTok = regexp.MustCompile(`(?i)\b(S\d{4}[A-Z0-9\-]*|CE\d{4,5}[A-Z0-9\-]*|USG[0-9A-Z\-]+|HiSecEngine[0-9A-Z\-]*|Eudemon[0-9A-Z\-]*|AR\d{3,4}[A-Z0-9\-]*|NE\d+[A-Z0-9\-]*|AC\d{4}[A-Z0-9\-]*|AirEngine[0-9A-Z\-]*)\b`)

	// Ruijie: RG-NBR-N7204-E(v1.11) / Ruijie ... RG-S5750-24GT/8SFP-P ... RGOS 11.4(1)B2 / ReyeeOS 1.x
	ruijieModelVerParen = regexp.MustCompile(`(?i)\b(RG-[A-Z0-9][A-Z0-9\-/]*)\s*\(\s*(v?\d+(?:\.\d+)+[A-Za-z0-9\-_]*)\s*\)`)
	ruijieModelTok      = regexp.MustCompile(`(?i)\b(RG-(?:NBR|EG|RSR|WALL|FW|NGAF|WS|AC|AP|NBS|CS|ES|IS|N|S)[A-Z0-9\-/]*)\b`)
	ruijieRGOSVer       = regexp.MustCompile(`(?i)\b((?:RGOS|ReyeeOS)\s+[0-9A-Za-z.\(\)\-_]+)`)
	ruijieVerGeneric    = regexp.MustCompile(`(?i)(?:version|ver)\s*[:=]?\s*(v?\d+(?:\.\d+)+[A-Za-z0-9\(\)\-_]*)`)

	// H3C: H3C Comware Platform Software, Software Version 7.1.070, Release 6326P01 ... H3C S5130S-28P-EI
	h3cModelTok = regexp.MustCompile(`(?i)\b(S\d{4}[A-Z0-9\-]*|MSR\d+[A-Z0-9\-]*|CR\d+[A-Z0-9\-]*|ER\d+[A-Z0-9\-]*|SR\d+[A-Z0-9\-]*|F\d{3,4}[A-Z0-9\-]*|WX\d+[A-Z0-9\-]*|SecPath\s*[A-Z0-9\-]+)\b`)
	h3cVerTok   = regexp.MustCompile(`(?i)(Version\s+[0-9.]+(?:,\s*Release\s+[0-9A-Za-z]+)?|Release\s+[0-9A-Za-z]+)`)

	// Cisco: Cisco IOS Software, C2960X Software ... Version 15.2(4)E7 / NX-OS / Catalyst C9300-24T
	ciscoModelTok = regexp.MustCompile(`(?i)\b(WS-C\d+[A-Z0-9\-]*|C\d{4}[A-Z0-9\-]*|N\dK-[A-Z0-9\-]+|ISR\d+[A-Z0-9\-]*|ASR\d+[A-Z0-9\-]*|CSR\d+[A-Z0-9\-]*|ASA\d+[A-Z0-9\-]*|FPR\d+[A-Z0-9\-]*|AIR-CT\d+[A-Z0-9\-]*)\b`)
	ciscoVerTok   = regexp.MustCompile(`(?i)Version\s+([0-9]+\.[0-9]+(?:\.[0-9A-Za-z]+|\([0-9A-Za-z]+\)[0-9A-Za-z.]*))`)

	// Juniper: Juniper Networks, Inc. ex4300-48p Ethernet Switch, kernel JUNOS 18.4R3
	juniperModelTok = regexp.MustCompile(`(?i)\b((?:EX|QFX|MX|SRX|PTX|ACX)\d+[A-Z0-9\-]*)\b`)
	juniperVerTok   = regexp.MustCompile(`(?i)JUNOS\s+([0-9A-Za-z.\-]+)`)

	// Fortinet: FortiGate-100F v7.2.4,build1396
	fortiModelTok = regexp.MustCompile(`(?i)\b(Forti(?:Gate|Switch|WiFi|AP)[A-Z0-9\-]*)\b`)
	fortiVerTok   = regexp.MustCompile(`(?i)\b(v?\d+\.\d+\.\d+(?:,build\d+)?)\b`)

	// MikroTik: RouterOS CCR1036-12G-4S / RouterOS 7.11.2 (RB4011iGS+)
	mikrotikModelTok = regexp.MustCompile(`(?i)\b(CCR\d+[A-Z0-9\-+]*|CRS\d+[A-Z0-9\-+]*|RB\d+[A-Z0-9\-+]*|CSS\d+[A-Z0-9\-+]*|hEX[A-Z0-9\-+]*|CHR)\b`)
	mikrotikVerTok   = regexp.MustCompile(`(?i)RouterOS\s+(?:v)?(\d+\.\d+(?:\.\d+)?)`)

	// Arista: Arista Networks EOS version 4.28.3M running on an Arista Networks DCS-7050SX3-48YC8
	aristaModelTok = regexp.MustCompile(`(?i)\b(DCS-[A-Z0-9\-]+|CCS-[A-Z0-9\-]+|vEOS)\b`)
	aristaVerTok   = regexp.MustCompile(`(?i)EOS\s+version\s+([0-9A-Za-z.\-]+)`)

	// ZTE: ZXR10 5950-52TM / ZROS V3.00.11
	zteModelTok = regexp.MustCompile(`(?i)\b(ZXR10\s+[A-Z0-9\-]+|ZXR10-[A-Z0-9\-]+)\b`)
	zteVerTok   = regexp.MustCompile(`(?i)\b(V\d+\.\d+(?:\.\d+)+[A-Za-z0-9]*)\b`)

	// Generic version fallback
	genericVerTok = regexp.MustCompile(`(?i)(?:version|ver)\s*[:=]?\s*(v?\d+\.\d+(?:\.\d+)*[A-Za-z0-9\-_]*)`)
)

// Identify 根据 sysObjectID 与 sysDescr 识别厂商/家族/型号/版本。
func Identify(sysOID, sysDescr string) Fingerprint {
	fp := Fingerprint{Vendor: "Unknown", Family: FamilyUnknown}
	oid := strings.TrimPrefix(strings.TrimSpace(sysOID), ".")
	const entPrefix = "1.3.6.1.4.1."
	if strings.HasPrefix(oid, entPrefix) {
		ent := strings.SplitN(strings.TrimPrefix(oid, entPrefix), ".", 2)[0]
		if !genericEnterpriseIDs[ent] {
			if v, ok := enterpriseVendors[ent]; ok {
				fp.Vendor = v
			}
		}
	}

	descr := strings.TrimSpace(sysDescr)
	lower := strings.ToLower(descr)
	if fp.Vendor == "Unknown" {
		fp.Vendor = inferVendorFromDescr(lower)
	}

	switch fp.Vendor {
	case "Huawei":
		identifyHuawei(&fp, descr, lower)
	case "Ruijie":
		identifyRuijie(&fp, oid, descr, lower)
	case "H3C":
		identifyH3C(&fp, descr, lower)
	case "Cisco":
		identifyCisco(&fp, descr, lower)
	case "Juniper":
		identifyJuniper(&fp, descr, lower)
	case "Fortinet":
		identifyFortinet(&fp, descr, lower)
	case "MikroTik":
		identifyMikroTik(&fp, descr, lower)
	case "Arista":
		identifyArista(&fp, descr, lower)
	case "ZTE":
		identifyZTE(&fp, descr, lower)
	default:
		if m := genericVerTok.FindStringSubmatch(descr); m != nil {
			fp.OSVersion = m[1]
		}
		fp.Family = genericFamily(lower)
	}
	return fp
}

// InferFamily 在已有厂商与型号(例如从 ENTITY-MIB 补全)后重新推断设备家族。
func InferFamily(vendor, model, sysDescr string) string {
	lower := strings.ToLower(sysDescr)
	switch vendor {
	case "Huawei":
		return huaweiFamily(model, lower)
	case "Ruijie":
		return ruijieFamily(model, lower)
	case "H3C":
		return h3cFamily(model, lower)
	case "Cisco":
		return ciscoFamily(model, lower)
	case "Juniper":
		return juniperFamily(model, lower)
	case "Fortinet":
		return fortinetFamily(model, lower)
	case "MikroTik":
		return mikrotikFamily(model, lower)
	case "Arista":
		return FamilySwitch
	default:
		return genericFamily(lower)
	}
}

func inferVendorFromDescr(lower string) string {
	switch {
	case strings.Contains(lower, "huawei") || strings.Contains(lower, "versatile routing platform"):
		return "Huawei"
	case strings.Contains(lower, "ruijie") || strings.Contains(lower, "reyee") || strings.Contains(lower, "rgos") || strings.Contains(lower, "rg-"):
		return "Ruijie"
	case strings.Contains(lower, "h3c") || strings.Contains(lower, "comware"):
		return "H3C"
	case strings.Contains(lower, "cisco") || strings.Contains(lower, "nx-os") || strings.Contains(lower, "catalyst"):
		return "Cisco"
	case strings.Contains(lower, "juniper") || strings.Contains(lower, "junos"):
		return "Juniper"
	case strings.Contains(lower, "fortigate") || strings.Contains(lower, "fortinet") || strings.Contains(lower, "fortiswitch"):
		return "Fortinet"
	case strings.Contains(lower, "mikrotik") || strings.Contains(lower, "routeros"):
		return "MikroTik"
	case strings.Contains(lower, "arista"):
		return "Arista"
	case strings.Contains(lower, "zte") || strings.Contains(lower, "zxr10"):
		return "ZTE"
	case strings.Contains(lower, "aruba"):
		return "Aruba"
	case strings.Contains(lower, "sangfor"):
		return "Sangfor"
	case strings.Contains(lower, "palo alto") || strings.Contains(lower, "pan-os"):
		return "PaloAlto"
	case strings.Contains(lower, "tp-link") || strings.Contains(lower, "tplink"):
		return "TP-Link"
	default:
		return "Unknown"
	}
}

func identifyHuawei(fp *Fingerprint, descr, lower string) {
	if m := huaweiModelVer.FindStringSubmatch(descr); m != nil {
		fp.Model, fp.OSVersion = m[1], m[2]
	}
	// 类似 "Quidway S9312 ... (S9300 V200R003C00SPC500)": 若前缀包含更具体的机型(如 S9312), 优先采用具体机型
	if specific := huaweiModelTok.FindString(descr); specific != "" {
		if fp.Model == "" || (strings.HasSuffix(fp.Model, "00") && specific != fp.Model) {
			fp.Model = specific
		}
	}
	fp.Family = huaweiFamily(fp.Model, lower)
}

// 锐捷 NBR/网关系列 sysObjectID 精确型号映射(真机校准)
var ruijieSysOIDModels = map[string]string{
	"1.3.6.1.4.1.4881.1.6.1.1.16": "RG-NBR-N7204-E",
}

func identifyRuijie(fp *Fingerprint, oid, descr, lower string) {
	if m := ruijieModelVerParen.FindStringSubmatch(descr); m != nil {
		fp.Model = strings.ToUpper(m[1])
		fp.OSVersion = m[2]
	}
	if fp.Model == "" {
		if m := ruijieModelTok.FindString(descr); m != "" {
			fp.Model = strings.ToUpper(m)
		}
	}
	if fp.Model == "" {
		if m, ok := ruijieSysOIDModels[oid]; ok {
			fp.Model = m
		}
	}
	if fp.OSVersion == "" {
		if m := ruijieRGOSVer.FindStringSubmatch(descr); m != nil {
			fp.OSVersion = strings.TrimSpace(m[1])
		} else if m := ruijieVerGeneric.FindStringSubmatch(descr); m != nil {
			fp.OSVersion = strings.TrimSpace(m[1])
		}
	}
	fp.Family = ruijieFamily(fp.Model, lower)
	if fp.Family == FamilyUnknown && strings.HasPrefix(oid, "1.3.6.1.4.1.4881.1.6.") {
		// 4881.1.6 为锐捷 NBR / 网关系列子树
		fp.Family = FamilyRouter
	}
}

func identifyH3C(fp *Fingerprint, descr, lower string) {
	if m := h3cModelTok.FindString(descr); m != "" {
		fp.Model = strings.TrimSpace(m)
	}
	if m := h3cVerTok.FindStringSubmatch(descr); m != nil {
		fp.OSVersion = strings.TrimSpace(m[1])
	}
	fp.Family = h3cFamily(fp.Model, lower)
}

func identifyCisco(fp *Fingerprint, descr, lower string) {
	// 优先选择最具体的型号(如 WS-C2960X-48FPS-L 优于 C2960X)
	for _, m := range ciscoModelTok.FindAllString(descr, -1) {
		if len(m) > len(fp.Model) {
			fp.Model = strings.ToUpper(m)
		}
	}
	if m := ciscoVerTok.FindStringSubmatch(descr); m != nil {
		fp.OSVersion = m[1]
	}
	fp.Family = ciscoFamily(fp.Model, lower)
}

func identifyJuniper(fp *Fingerprint, descr, lower string) {
	if m := juniperModelTok.FindString(descr); m != "" {
		fp.Model = strings.ToUpper(m)
	}
	if m := juniperVerTok.FindStringSubmatch(descr); m != nil {
		fp.OSVersion = "JUNOS " + m[1]
	}
	fp.Family = juniperFamily(fp.Model, lower)
}

func identifyFortinet(fp *Fingerprint, descr, lower string) {
	if m := fortiModelTok.FindString(descr); m != "" {
		fp.Model = m
	}
	if m := fortiVerTok.FindStringSubmatch(descr); m != nil {
		fp.OSVersion = m[1]
	}
	fp.Family = fortinetFamily(fp.Model, lower)
}

func identifyMikroTik(fp *Fingerprint, descr, lower string) {
	if m := mikrotikModelTok.FindString(descr); m != "" {
		fp.Model = m
	}
	if m := mikrotikVerTok.FindStringSubmatch(descr); m != nil {
		fp.OSVersion = "RouterOS " + m[1]
	}
	fp.Family = mikrotikFamily(fp.Model, lower)
}

func identifyArista(fp *Fingerprint, descr, lower string) {
	if m := aristaModelTok.FindString(descr); m != "" {
		fp.Model = m
	}
	if m := aristaVerTok.FindStringSubmatch(descr); m != nil {
		fp.OSVersion = "EOS " + m[1]
	}
	fp.Family = FamilySwitch
}

func identifyZTE(fp *Fingerprint, descr, lower string) {
	if m := zteModelTok.FindString(descr); m != "" {
		fp.Model = strings.TrimSpace(m)
	}
	if m := zteVerTok.FindStringSubmatch(descr); m != nil {
		fp.OSVersion = m[1]
	}
	fp.Family = genericFamily(lower)
	if fp.Family == FamilyUnknown && strings.HasPrefix(strings.ToUpper(fp.Model), "ZXR10") {
		fp.Family = FamilySwitch
	}
}

func huaweiFamily(model, lowerDescr string) string {
	m := strings.ToUpper(model)
	switch {
	case strings.HasPrefix(m, "USG"), strings.HasPrefix(m, "EUDEMON"), strings.HasPrefix(m, "HISECENGINE"):
		return FamilyFirewall
	case strings.HasPrefix(m, "AR"), strings.HasPrefix(m, "NE"):
		return FamilyRouter
	case strings.HasPrefix(m, "AIRENGINE"), regexp.MustCompile(`^AC\d`).MatchString(m):
		return FamilyWLANAC
	case strings.HasPrefix(m, "S"), strings.HasPrefix(m, "CE"):
		return FamilySwitch
	}
	return genericFamily(lowerDescr)
}

func ruijieFamily(model, lowerDescr string) string {
	m := strings.ToUpper(model)
	switch {
	case strings.HasPrefix(m, "RG-WALL"), strings.HasPrefix(m, "RG-FW"), strings.HasPrefix(m, "RG-NGAF"):
		return FamilyFirewall
	case strings.HasPrefix(m, "RG-NBR"), strings.HasPrefix(m, "RG-EG"), strings.HasPrefix(m, "RG-RSR"):
		return FamilyRouter
	case strings.HasPrefix(m, "RG-WS"), strings.HasPrefix(m, "RG-AC"):
		return FamilyWLANAC
	case strings.HasPrefix(m, "RG-S"), strings.HasPrefix(m, "RG-NBS"), strings.HasPrefix(m, "RG-CS"),
		strings.HasPrefix(m, "RG-ES"), strings.HasPrefix(m, "RG-IS"), strings.HasPrefix(m, "RG-N"):
		return FamilySwitch
	}
	if strings.Contains(lowerDescr, "gateway") || strings.Contains(lowerDescr, "nbr") {
		return FamilyRouter
	}
	return genericFamily(lowerDescr)
}

func h3cFamily(model, lowerDescr string) string {
	m := strings.ToUpper(model)
	switch {
	case strings.HasPrefix(m, "F"), strings.HasPrefix(m, "SECPATH"):
		return FamilyFirewall
	case strings.HasPrefix(m, "MSR"), strings.HasPrefix(m, "CR"), strings.HasPrefix(m, "ER"), strings.HasPrefix(m, "SR"):
		return FamilyRouter
	case strings.HasPrefix(m, "WX"):
		return FamilyWLANAC
	case strings.HasPrefix(m, "S"):
		return FamilySwitch
	}
	return genericFamily(lowerDescr)
}

func ciscoFamily(model, lowerDescr string) string {
	m := strings.ToUpper(model)
	switch {
	case strings.HasPrefix(m, "ASA"), strings.HasPrefix(m, "FPR"), strings.HasPrefix(m, "FTD"):
		return FamilyFirewall
	case strings.HasPrefix(m, "ISR"), strings.HasPrefix(m, "ASR"), strings.HasPrefix(m, "CSR"):
		return FamilyRouter
	case strings.HasPrefix(m, "AIR-CT"), strings.HasPrefix(m, "C9800"):
		return FamilyWLANAC
	case strings.HasPrefix(m, "WS-C"), strings.HasPrefix(m, "C2"), strings.HasPrefix(m, "C3"),
		strings.HasPrefix(m, "C9"), strings.HasPrefix(m, "N3K"), strings.HasPrefix(m, "N5K"),
		strings.HasPrefix(m, "N7K"), strings.HasPrefix(m, "N9K"):
		return FamilySwitch
	}
	return genericFamily(lowerDescr)
}

func juniperFamily(model, lowerDescr string) string {
	m := strings.ToUpper(model)
	switch {
	case strings.HasPrefix(m, "SRX"):
		return FamilyFirewall
	case strings.HasPrefix(m, "MX"), strings.HasPrefix(m, "PTX"), strings.HasPrefix(m, "ACX"):
		return FamilyRouter
	case strings.HasPrefix(m, "EX"), strings.HasPrefix(m, "QFX"):
		return FamilySwitch
	}
	return genericFamily(lowerDescr)
}

func fortinetFamily(model, lowerDescr string) string {
	m := strings.ToUpper(model)
	switch {
	case strings.HasPrefix(m, "FORTISWITCH"):
		return FamilySwitch
	case strings.HasPrefix(m, "FORTIGATE"), strings.HasPrefix(m, "FORTIWIFI"):
		return FamilyFirewall
	}
	if strings.Contains(lowerDescr, "switch") {
		return FamilySwitch
	}
	return FamilyFirewall
}

func mikrotikFamily(model, lowerDescr string) string {
	m := strings.ToUpper(model)
	if strings.HasPrefix(m, "CRS") || strings.HasPrefix(m, "CSS") {
		return FamilySwitch
	}
	if strings.HasPrefix(m, "CCR") || strings.HasPrefix(m, "RB") || strings.HasPrefix(m, "HEX") || m == "CHR" {
		return FamilyRouter
	}
	return genericFamily(lowerDescr)
}

func genericFamily(lowerDescr string) string {
	switch {
	case strings.Contains(lowerDescr, "firewall") || strings.Contains(lowerDescr, "fortigate") || strings.Contains(lowerDescr, "asa") || strings.Contains(lowerDescr, "usg") || strings.Contains(lowerDescr, "ngaf"):
		return FamilyFirewall
	case strings.Contains(lowerDescr, "wlan") && strings.Contains(lowerDescr, "controller"):
		return FamilyWLANAC
	case strings.Contains(lowerDescr, "switch"):
		return FamilySwitch
	case strings.Contains(lowerDescr, "router") || strings.Contains(lowerDescr, "gateway"):
		return FamilyRouter
	default:
		return FamilyUnknown
	}
}

// RoleByFamily 由设备家族给出初始角色;交换机的 core/aggregation/access 由拓扑位置进一步判定。
func RoleByFamily(family string) string {
	switch family {
	case FamilyFirewall:
		return RoleFirewall
	case FamilyRouter:
		return RoleRouter
	case FamilyWLANAC:
		return RoleAC
	case FamilySwitch:
		return RoleAggregation
	}
	return RoleUnknown
}
