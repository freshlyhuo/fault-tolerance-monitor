/*包括三个核心函数：
ParsePDU(frame []byte) → PDUEvent:判断 PDU 类型（Meta/FileData/EOF/Error）
	按协议字段偏移解码字段：SessionID,Chunk offset,Packet length,Error code,Timestamp 等

ExtractBusinessMetrics(pdu PDUEvent) → BusinessMetric:提取业务层健康指标，如：
文件序号,偏移跳变情况,校验值,丢包统计

GenerateBusinessError(err error) → BusinessMetric:对“CRC 错误 / 长度错误 / PDU 解码错误”等生成异常指标
*/
package business

import (
    "encoding/binary"
    "errors"
    "fmt"

    "health-monitor/pkg/models"
)

// ParsePDU 统一解析入口
// 输入完整 frame（已由组帧器校验长度、CRC）
// 返回 models.PDUEvent
func ParsePDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 3 {
        return nil, errors.New("frame too short")
    }

    pduType := models.PDUType(frame[0])

    switch pduType {

    // -----------------------------
    // 数据类 PDU
    // -----------------------------
    case models.PDUTypeDataFile, models.PDUTypeConfigFile, models.PDUTypePackageFile:
        return parseFileDataPDU(frame)

    // -----------------------------
    // 指令类 PDU
    // -----------------------------
    case models.PDUTypeCommand:
        return parseCommandPDU(frame)
    }

    return nil, fmt.Errorf("unknown PDU type: 0x%02X", frame[0])
}

////////////////////////////////////////////////////////////////////////////////
// 1. File Data PDU (数据/配置/软件包 文件数据)
////////////////////////////////////////////////////////////////////////////////

func parseFileDataPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 12 {
        return nil, errors.New("FileDataPDU too short")
    }

    p := &models.FileDataPDU{}
    p.Type = models.PDUType(frame[0])
    p.RawLength = len(frame)

    p.SessionID = binary.BigEndian.Uint32(frame[1:5])
    p.Offset = binary.BigEndian.Uint16(frame[5:7])
    p.DataLength = binary.BigEndian.Uint16(frame[7:9])

    dataEnd := 9 + int(p.DataLength)
    if dataEnd > len(frame)-2 {
        return nil, errors.New("FileDataPDU data length mismatch")
    }

    p.Data = frame[9:dataEnd]
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}

////////////////////////////////////////////////////////////////////////////////
// 2. Command PDU 统一入口
////////////////////////////////////////////////////////////////////////////////

func parseCommandPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 3 {
        return nil, errors.New("Command PDU too short")
    }

    subtype := models.CommandSubtype(frame[1])

    switch subtype {

    case models.CmdMetaPDU:
        return parseMetaPDU(frame)

    case models.CmdMetaFailPDU:
        return parseMetaFailPDU(frame)

    case models.CmdEOF:
        return parseEOFPDU(frame)

    case models.CmdTransferCompleted:
        return parseTransferCompletedPDU(frame)

    case models.CmdTransferFailed:
        return parseTransferFailedPDU(frame)

    case models.CmdResumePoint:
        return parseResumePointPDU(frame)

    case models.CmdNAK:
        return parseNAKPDU(frame)

    case models.CmdStorageQuery:
        return parseStorageQueryPDU(frame)

    case models.CmdStorageQueryResp:
        return parseStorageQueryRespPDU(frame)

    case models.CmdDeleteFile:
        return parseDeleteFilePDU(frame)

    case models.CmdDeleteFileResp:
        return parseDeleteFileRespPDU(frame)

    case models.CmdQueryFileList:
        return parseQueryFileListPDU(frame)

    case models.CmdQueryFileListResp:
        return parseQueryFileListRespPDU(frame)

    case models.CmdDownloadFile:
        return parseDownloadFilePDU(frame)

    case models.CmdDeltaUpgrade:
        return parseDeltaUpgradePDU(frame)

    case models.CmdUpgradeSuccess:
        return parseUpgradeSuccessPDU(frame)

    case models.CmdUpgradeFailed:
        return parseUpgradeFailedPDU(frame)
    }

    return nil, fmt.Errorf("unknown command subtype: 0x%02X", subtype)
}

////////////////////////////////////////////////////////////////////////////////
// 以下为每一种指令 PDU 的具体解析函数
////////////////////////////////////////////////////////////////////////////////

// ------------------------
// 元数据 PDU
// ------------------------
func parseMetaPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 15 {
        return nil, errors.New("MetaPDU too short")
    }

    p := &models.MetaPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.SessionID = binary.BigEndian.Uint32(frame[2:6])
    nameLen := frame[6]

    // 文件名
    nameStart := 7
    nameEnd := nameStart + int(nameLen)

    if nameEnd > len(frame) {
        return nil, errors.New("MetaPDU invalid filename length")
    }
    p.FileName = string(frame[nameStart:nameEnd])

    // 文件长度 4 字节
    idx := nameEnd
    p.FileLength = binary.BigEndian.Uint32(frame[idx : idx+4])
    idx += 4

    // 文件类型 1 字节
    p.FileType = frame[idx]
    idx++

    // 软件 ID 8 字节
    p.SoftwareID = binary.BigEndian.Uint64(frame[idx : idx+8])
    idx += 8

    // 软件版本号 4 字节
    p.SoftwareVer = binary.BigEndian.Uint32(frame[idx : idx+4])
    idx += 4

    // 数字签名 8 字节
    p.Signature = make([]byte, 8)
    copy(p.Signature, frame[idx:idx+8])
    idx += 8

    // 最后两字节 CRC
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}

// ----------------------------
// 元数据协商失败
// ----------------------------
func parseMetaFailPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 11 {
        return nil, errors.New("MetaFailPDU too short")
    }

    p := &models.MetaFailPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.SessionID = binary.BigEndian.Uint32(frame[2:6])
    p.ErrorCode = frame[6]
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}

// ----------------------------
// EOF PDU
// ----------------------------
func parseEOFPDU(frame []byte) (models.PDUEvent, error) {
    if len(frame) < 15 {
        return nil, errors.New("EOFPDU too short")
    }

    p := &models.EOFPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.SessionID = binary.BigEndian.Uint32(frame[2:6])
    p.Status = frame[6]
    p.FinalSize = binary.BigEndian.Uint32(frame[7:11])
    p.FileChecksum = binary.BigEndian.Uint16(frame[11:13])
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}

// ----------------------------
// Transfer Completed
// ----------------------------
func parseTransferCompletedPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 23 {
        return nil, errors.New("TransferCompletedPDU too short")
    }

    p := &models.TransferCompletedPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.SessionID = binary.BigEndian.Uint32(frame[2:6])
    p.FileSize = binary.BigEndian.Uint32(frame[6:10])
    p.Checksum = binary.BigEndian.Uint16(frame[10:12])
    p.Timestamp = binary.BigEndian.Uint64(frame[12:20])
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}

// ----------------------------
// Transfer Failed
// ----------------------------
func parseTransferFailedPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 18 {
        return nil, errors.New("TransferFailedPDU too short")
    }

    p := &models.TransferFailedPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.SessionID = binary.BigEndian.Uint32(frame[2:6])
    p.ErrorCode = frame[6]
    p.Timestamp = binary.BigEndian.Uint64(frame[7:15])
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}

// ----------------------------
// Resume Point
// ----------------------------
func parseResumePointPDU(frame []byte) (models.PDUEvent, error) {
    if len(frame) < 11 {
        return nil, errors.New("ResumePointPDU too short")
    }

    p := &models.ResumePointPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.SessionID = binary.BigEndian.Uint32(frame[2:6])
    p.NextOffset = binary.BigEndian.Uint16(frame[6:8])
    p.CurrentSize = binary.BigEndian.Uint16(frame[8:10])
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}

// ----------------------------
// NAK PDU
// ----------------------------
func parseNAKPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 9 {
        return nil, errors.New("NAKPDU too short")
    }

    p := &models.NAKPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.SessionID = binary.BigEndian.Uint32(frame[2:6])
    p.Offset = frame[6]
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}

// ----------------------------
// Storage Query
// ----------------------------
func parseStorageQueryPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 20 {
        return nil, errors.New("StorageQueryPDU too short")
    }

    p := &models.StorageQueryPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.CommandID = binary.BigEndian.Uint64(frame[2:10])
    p.Signature = make([]byte, 8)
    copy(p.Signature, frame[10:18])
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}

// ----------------------------
// Storage Query Response
// ----------------------------
func parseStorageQueryRespPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 23 {
        return nil, errors.New("StorageQueryRespPDU too short")
    }

    p := &models.StorageQueryRespPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.CommandID = binary.BigEndian.Uint64(frame[2:10])
    p.Status = frame[10]
    p.PartitionCount = frame[11]

    idx := 12
    p.Partitions = make([]models.PartitionInfo, p.PartitionCount)

    for i := 0; i < int(p.PartitionCount); i++ {
        if idx+9 > len(frame)-2 {
            return nil, errors.New("StorageQueryRespPDU partition data overflow")
        }
        p.Partitions[i].PartitionID = frame[idx]
        p.Partitions[i].TotalSpace = binary.BigEndian.Uint32(frame[idx+1 : idx+5])
        p.Partitions[i].FreeSpace = binary.BigEndian.Uint32(frame[idx+5 : idx+9])
        idx += 9
    }

    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])
    return p, nil
}

// ----------------------------
// Delete File
// ----------------------------
func parseDeleteFilePDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 15 {
        return nil, errors.New("DeleteFilePDU too short")
    }

    p := &models.DeleteFilePDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.CommandID = binary.BigEndian.Uint64(frame[2:10])

    nameLen := frame[10]
    nameStart := 11
    nameEnd := nameStart + int(nameLen)
    if nameEnd > len(frame) {
        return nil, errors.New("DeleteFilePDU invalid filename length")
    }
    p.FileName = string(frame[nameStart:nameEnd])

    idx := nameEnd
    if idx+2+8+2 > len(frame) {
        return nil, errors.New("DeleteFilePDU length mismatch")
    }

    p.FileHash = binary.BigEndian.Uint16(frame[idx : idx+2])
    idx += 2

    p.Signature = make([]byte, 8)
    copy(p.Signature, frame[idx:idx+8])
    idx += 8

    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])
    return p, nil
}

// ----------------------------
// Delete File Response
// ----------------------------
func parseDeleteFileRespPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 17 {
        return nil, errors.New("DeleteFileRespPDU too short")
    }

    p := &models.DeleteFileRespPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.CommandID = binary.BigEndian.Uint64(frame[2:10])
    p.Status = frame[10]
    p.ActualHash = binary.BigEndian.Uint16(frame[11:13])
    p.RequestHash = binary.BigEndian.Uint16(frame[13:15])
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}

// ----------------------------
// Query File List
// ----------------------------
func parseQueryFileListPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 30 {
        return nil, errors.New("QueryFileListPDU too short")
    }

    p := &models.QueryFileListPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.CommandID = binary.BigEndian.Uint64(frame[2:10])
    p.Directory = binary.BigEndian.Uint32(frame[10:14])
    p.TargetName = binary.BigEndian.Uint32(frame[14:18])

    p.Signature = make([]byte, 8)
    copy(p.Signature, frame[18:26])

    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])
    return p, nil
}

// ----------------------------
// Query File List Response
// ----------------------------
func parseQueryFileListRespPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 23 {
        return nil, errors.New("QueryFileListRespPDU too short")
    }

    p := &models.QueryFileListRespPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.CommandID = binary.BigEndian.Uint64(frame[2:10])
    p.Status = frame[10]
    p.Directory = binary.BigEndian.Uint32(frame[11:15])
    p.TargetName = binary.BigEndian.Uint32(frame[15:19])
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}

// ----------------------------
// 下载文件指令
// ----------------------------
func parseDownloadFilePDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 15 {
        return nil, errors.New("DownloadFilePDU too short")
    }

    p := &models.DownloadFilePDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.CommandID = binary.BigEndian.Uint64(frame[2:10])
    nameLen := frame[10]
    nameStart := 11
    nameEnd := nameStart + int(nameLen)

    if nameEnd > len(frame) {
        return nil, errors.New("DownloadFilePDU filename overflow")
    }

    p.FileName = string(frame[nameStart:nameEnd])
    idx := nameEnd

    // 起始偏移量
    p.Offset = binary.BigEndian.Uint16(frame[idx : idx+2])
    idx += 2

    // 数字签名（8 bytes）
    p.Signature = make([]byte, 8)
    copy(p.Signature, frame[idx:idx+8])
    idx += 8

    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])
    return p, nil
}

// ----------------------------
// 差分升级指令
// ----------------------------
func parseDeltaUpgradePDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 32 {
        return nil, errors.New("DeltaUpgradePDU too short")
    }

    p := &models.DeltaUpgradePDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.CommandID = binary.BigEndian.Uint64(frame[2:10])
    p.SoftwareID = binary.BigEndian.Uint64(frame[10:18])
    p.TargetVer = binary.BigEndian.Uint32(frame[18:22])

    p.Signature = make([]byte, 8)
    copy(p.Signature, frame[22:30])

    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])
    return p, nil
}

// ----------------------------
// 升级成功报告
// ----------------------------
func parseUpgradeSuccessPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 32 {
        return nil, errors.New("UpgradeSuccessPDU too short")
    }

    p := &models.UpgradeSuccessPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.CommandID = binary.BigEndian.Uint64(frame[2:10])
    p.SoftwareID = binary.BigEndian.Uint64(frame[10:18])
    p.NewVersion = binary.BigEndian.Uint32(frame[18:22])
    p.Timestamp = binary.BigEndian.Uint64(frame[22:30])
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])
    return p, nil
}

// ----------------------------
// 升级失败报告
// ----------------------------
func parseUpgradeFailedPDU(frame []byte) (models.PDUEvent, error) {

    if len(frame) < 33 {
        return nil, errors.New("UpgradeFailedPDU too short")
    }

    p := &models.UpgradeFailedPDU{}
    p.Type = models.PDUTypeCommand
    p.RawLength = len(frame)

    p.CommandID = binary.BigEndian.Uint64(frame[2:10])
    p.SoftwareID = binary.BigEndian.Uint64(frame[10:18])
    p.TargetVer = binary.BigEndian.Uint32(frame[18:22])
    p.FailStage = frame[22]
    p.ErrorCode = frame[23]
    p.Timestamp = binary.BigEndian.Uint64(frame[24:32])
    p.CRC = binary.BigEndian.Uint16(frame[len(frame)-2:])

    return p, nil
}
