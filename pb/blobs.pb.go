// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.32.0
// 	protoc        v4.25.1
// source: blobs.proto

package pbbmsrv

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Blobs struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Blobs []*Blob `protobuf:"bytes,1,rep,name=blobs,proto3" json:"blobs"`
}

func (x *Blobs) Reset() {
	*x = Blobs{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobs_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Blobs) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Blobs) ProtoMessage() {}

func (x *Blobs) ProtoReflect() protoreflect.Message {
	mi := &file_blobs_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Blobs.ProtoReflect.Descriptor instead.
func (*Blobs) Descriptor() ([]byte, []int) {
	return file_blobs_proto_rawDescGZIP(), []int{0}
}

func (x *Blobs) GetBlobs() []*Blob {
	if x != nil {
		return x.Blobs
	}
	return nil
}

type Blob struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Index                       uint32             `protobuf:"varint,1,opt,name=index,proto3" json:"index"`
	Blob                        []byte             `protobuf:"bytes,2,opt,name=blob,proto3" json:"blob"`
	KzgCommitment               []byte             `protobuf:"bytes,3,opt,name=kzg_commitment,json=kzgCommitment,proto3" json:"kzg_commitment"`
	KzgProof                    []byte             `protobuf:"bytes,4,opt,name=kzg_proof,json=kzgProof,proto3" json:"kzg_proof"`
	SignedBlockHeader           *SignedBlockHeader `protobuf:"bytes,5,opt,name=signed_block_header,json=signedBlockHeader,proto3" json:"signed_block_header"`
	KzgCommitmentInclusionProof [][]byte           `protobuf:"bytes,6,rep,name=kzg_commitment_inclusion_proof,json=kzgCommitmentInclusionProof,proto3" json:"kzg_commitment_inclusion_proof"`
	Extra                       *Extra             `protobuf:"bytes,7,opt,name=extra,proto3" json:"extra"`
}

func (x *Blob) Reset() {
	*x = Blob{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobs_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Blob) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Blob) ProtoMessage() {}

func (x *Blob) ProtoReflect() protoreflect.Message {
	mi := &file_blobs_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Blob.ProtoReflect.Descriptor instead.
func (*Blob) Descriptor() ([]byte, []int) {
	return file_blobs_proto_rawDescGZIP(), []int{1}
}

func (x *Blob) GetIndex() uint32 {
	if x != nil {
		return x.Index
	}
	return 0
}

func (x *Blob) GetBlob() []byte {
	if x != nil {
		return x.Blob
	}
	return nil
}

func (x *Blob) GetKzgCommitment() []byte {
	if x != nil {
		return x.KzgCommitment
	}
	return nil
}

func (x *Blob) GetKzgProof() []byte {
	if x != nil {
		return x.KzgProof
	}
	return nil
}

func (x *Blob) GetSignedBlockHeader() *SignedBlockHeader {
	if x != nil {
		return x.SignedBlockHeader
	}
	return nil
}

func (x *Blob) GetKzgCommitmentInclusionProof() [][]byte {
	if x != nil {
		return x.KzgCommitmentInclusionProof
	}
	return nil
}

func (x *Blob) GetExtra() *Extra {
	if x != nil {
		return x.Extra
	}
	return nil
}

type SignedBlockHeader struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Message   *Message `protobuf:"bytes,1,opt,name=message,proto3" json:"message"`
	Signature []byte   `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature"`
}

func (x *SignedBlockHeader) Reset() {
	*x = SignedBlockHeader{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobs_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignedBlockHeader) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignedBlockHeader) ProtoMessage() {}

func (x *SignedBlockHeader) ProtoReflect() protoreflect.Message {
	mi := &file_blobs_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignedBlockHeader.ProtoReflect.Descriptor instead.
func (*SignedBlockHeader) Descriptor() ([]byte, []int) {
	return file_blobs_proto_rawDescGZIP(), []int{2}
}

func (x *SignedBlockHeader) GetMessage() *Message {
	if x != nil {
		return x.Message
	}
	return nil
}

func (x *SignedBlockHeader) GetSignature() []byte {
	if x != nil {
		return x.Signature
	}
	return nil
}

type Message struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Slot          uint64 `protobuf:"varint,1,opt,name=slot,proto3" json:"slot"`
	ProposerIndex uint64 `protobuf:"varint,2,opt,name=proposer_index,json=proposerIndex,proto3" json:"proposer_index"`
	ParentRoot    []byte `protobuf:"bytes,3,opt,name=parent_root,json=parentRoot,proto3" json:"parent_root"`
	StateRoot     []byte `protobuf:"bytes,4,opt,name=state_root,json=stateRoot,proto3" json:"state_root"`
	BodyRoot      []byte `protobuf:"bytes,5,opt,name=body_root,json=bodyRoot,proto3" json:"body_root"`
}

func (x *Message) Reset() {
	*x = Message{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobs_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Message) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message) ProtoMessage() {}

func (x *Message) ProtoReflect() protoreflect.Message {
	mi := &file_blobs_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Message.ProtoReflect.Descriptor instead.
func (*Message) Descriptor() ([]byte, []int) {
	return file_blobs_proto_rawDescGZIP(), []int{3}
}

func (x *Message) GetSlot() uint64 {
	if x != nil {
		return x.Slot
	}
	return 0
}

func (x *Message) GetProposerIndex() uint64 {
	if x != nil {
		return x.ProposerIndex
	}
	return 0
}

func (x *Message) GetParentRoot() []byte {
	if x != nil {
		return x.ParentRoot
	}
	return nil
}

func (x *Message) GetStateRoot() []byte {
	if x != nil {
		return x.StateRoot
	}
	return nil
}

func (x *Message) GetBodyRoot() []byte {
	if x != nil {
		return x.BodyRoot
	}
	return nil
}

type Extra struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlockNumber uint64                 `protobuf:"varint,1,opt,name=block_number,json=blockNumber,proto3" json:"block_number"`
	Timestamp   *timestamppb.Timestamp `protobuf:"bytes,2,opt,name=timestamp,proto3" json:"timestamp"`
}

func (x *Extra) Reset() {
	*x = Extra{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blobs_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Extra) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Extra) ProtoMessage() {}

func (x *Extra) ProtoReflect() protoreflect.Message {
	mi := &file_blobs_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Extra.ProtoReflect.Descriptor instead.
func (*Extra) Descriptor() ([]byte, []int) {
	return file_blobs_proto_rawDescGZIP(), []int{4}
}

func (x *Extra) GetBlockNumber() uint64 {
	if x != nil {
		return x.BlockNumber
	}
	return 0
}

func (x *Extra) GetTimestamp() *timestamppb.Timestamp {
	if x != nil {
		return x.Timestamp
	}
	return nil
}

var File_blobs_proto protoreflect.FileDescriptor

var file_blobs_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x62, 0x6c, 0x6f, 0x62, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x17, 0x70,
	0x69, 0x6e, 0x61, 0x78, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x62, 0x6c,
	0x6f, 0x62, 0x73, 0x2e, 0x76, 0x31, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x3c, 0x0a, 0x05, 0x42, 0x6c, 0x6f, 0x62, 0x73,
	0x12, 0x33, 0x0a, 0x05, 0x62, 0x6c, 0x6f, 0x62, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x1d, 0x2e, 0x70, 0x69, 0x6e, 0x61, 0x78, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d,
	0x2e, 0x62, 0x6c, 0x6f, 0x62, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x05,
	0x62, 0x6c, 0x6f, 0x62, 0x73, 0x22, 0xcb, 0x02, 0x0a, 0x04, 0x42, 0x6c, 0x6f, 0x62, 0x12, 0x14,
	0x0a, 0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x69,
	0x6e, 0x64, 0x65, 0x78, 0x12, 0x12, 0x0a, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x12, 0x25, 0x0a, 0x0e, 0x6b, 0x7a, 0x67, 0x5f,
	0x63, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x0d, 0x6b, 0x7a, 0x67, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x12,
	0x1b, 0x0a, 0x09, 0x6b, 0x7a, 0x67, 0x5f, 0x70, 0x72, 0x6f, 0x6f, 0x66, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x08, 0x6b, 0x7a, 0x67, 0x50, 0x72, 0x6f, 0x6f, 0x66, 0x12, 0x5a, 0x0a, 0x13,
	0x73, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x68, 0x65, 0x61,
	0x64, 0x65, 0x72, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2a, 0x2e, 0x70, 0x69, 0x6e, 0x61,
	0x78, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x62, 0x6c, 0x6f, 0x62, 0x73,
	0x2e, 0x76, 0x31, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48,
	0x65, 0x61, 0x64, 0x65, 0x72, 0x52, 0x11, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x42, 0x6c, 0x6f,
	0x63, 0x6b, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x43, 0x0a, 0x1e, 0x6b, 0x7a, 0x67, 0x5f,
	0x63, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x5f, 0x69, 0x6e, 0x63, 0x6c, 0x75,
	0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x70, 0x72, 0x6f, 0x6f, 0x66, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0c,
	0x52, 0x1b, 0x6b, 0x7a, 0x67, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x49,
	0x6e, 0x63, 0x6c, 0x75, 0x73, 0x69, 0x6f, 0x6e, 0x50, 0x72, 0x6f, 0x6f, 0x66, 0x12, 0x34, 0x0a,
	0x05, 0x65, 0x78, 0x74, 0x72, 0x61, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x70,
	0x69, 0x6e, 0x61, 0x78, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x62, 0x6c,
	0x6f, 0x62, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x45, 0x78, 0x74, 0x72, 0x61, 0x52, 0x05, 0x65, 0x78,
	0x74, 0x72, 0x61, 0x22, 0x6d, 0x0a, 0x11, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x42, 0x6c, 0x6f,
	0x63, 0x6b, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x3a, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x20, 0x2e, 0x70, 0x69, 0x6e, 0x61,
	0x78, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x62, 0x6c, 0x6f, 0x62, 0x73,
	0x2e, 0x76, 0x31, 0x2e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x07, 0x6d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75,
	0x72, 0x65, 0x22, 0xa1, 0x01, 0x0a, 0x07, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x12,
	0x0a, 0x04, 0x73, 0x6c, 0x6f, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x73, 0x6c,
	0x6f, 0x74, 0x12, 0x25, 0x0a, 0x0e, 0x70, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x65, 0x72, 0x5f, 0x69,
	0x6e, 0x64, 0x65, 0x78, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0d, 0x70, 0x72, 0x6f, 0x70,
	0x6f, 0x73, 0x65, 0x72, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x12, 0x1f, 0x0a, 0x0b, 0x70, 0x61, 0x72,
	0x65, 0x6e, 0x74, 0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0a,
	0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x74,
	0x61, 0x74, 0x65, 0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x09,
	0x73, 0x74, 0x61, 0x74, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x62, 0x6f, 0x64,
	0x79, 0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x62, 0x6f,
	0x64, 0x79, 0x52, 0x6f, 0x6f, 0x74, 0x22, 0x64, 0x0a, 0x05, 0x45, 0x78, 0x74, 0x72, 0x61, 0x12,
	0x21, 0x0a, 0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75, 0x6d, 0x62,
	0x65, 0x72, 0x12, 0x38, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x42, 0x42, 0x5a, 0x40,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x69, 0x6e, 0x61, 0x78,
	0x2d, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x2f, 0x62, 0x6c, 0x6f, 0x62, 0x73, 0x2d, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x70, 0x62, 0x2f, 0x70, 0x69, 0x6e, 0x61, 0x78, 0x2f,
	0x62, 0x6c, 0x6f, 0x62, 0x73, 0x2f, 0x76, 0x31, 0x3b, 0x70, 0x62, 0x62, 0x6d, 0x73, 0x72, 0x76,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_blobs_proto_rawDescOnce sync.Once
	file_blobs_proto_rawDescData = file_blobs_proto_rawDesc
)

func file_blobs_proto_rawDescGZIP() []byte {
	file_blobs_proto_rawDescOnce.Do(func() {
		file_blobs_proto_rawDescData = protoimpl.X.CompressGZIP(file_blobs_proto_rawDescData)
	})
	return file_blobs_proto_rawDescData
}

var file_blobs_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_blobs_proto_goTypes = []interface{}{
	(*Blobs)(nil),                 // 0: pinax.ethereum.blobs.v1.Blobs
	(*Blob)(nil),                  // 1: pinax.ethereum.blobs.v1.Blob
	(*SignedBlockHeader)(nil),     // 2: pinax.ethereum.blobs.v1.SignedBlockHeader
	(*Message)(nil),               // 3: pinax.ethereum.blobs.v1.Message
	(*Extra)(nil),                 // 4: pinax.ethereum.blobs.v1.Extra
	(*timestamppb.Timestamp)(nil), // 5: google.protobuf.Timestamp
}
var file_blobs_proto_depIdxs = []int32{
	1, // 0: pinax.ethereum.blobs.v1.Blobs.blobs:type_name -> pinax.ethereum.blobs.v1.Blob
	2, // 1: pinax.ethereum.blobs.v1.Blob.signed_block_header:type_name -> pinax.ethereum.blobs.v1.SignedBlockHeader
	4, // 2: pinax.ethereum.blobs.v1.Blob.extra:type_name -> pinax.ethereum.blobs.v1.Extra
	3, // 3: pinax.ethereum.blobs.v1.SignedBlockHeader.message:type_name -> pinax.ethereum.blobs.v1.Message
	5, // 4: pinax.ethereum.blobs.v1.Extra.timestamp:type_name -> google.protobuf.Timestamp
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_blobs_proto_init() }
func file_blobs_proto_init() {
	if File_blobs_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_blobs_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Blobs); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobs_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Blob); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobs_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignedBlockHeader); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobs_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Message); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_blobs_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Extra); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_blobs_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_blobs_proto_goTypes,
		DependencyIndexes: file_blobs_proto_depIdxs,
		MessageInfos:      file_blobs_proto_msgTypes,
	}.Build()
	File_blobs_proto = out.File
	file_blobs_proto_rawDesc = nil
	file_blobs_proto_goTypes = nil
	file_blobs_proto_depIdxs = nil
}