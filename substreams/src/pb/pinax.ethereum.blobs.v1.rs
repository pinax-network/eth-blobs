// @generated
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Blobs {
    #[prost(message, repeated, tag="1")]
    pub blobs: ::prost::alloc::vec::Vec<Blob>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Blob {
    #[prost(uint32, tag="1")]
    pub index: u32,
    #[prost(bytes="vec", tag="2")]
    pub blob: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="3")]
    pub kzg_commitment: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="4")]
    pub kzg_proof: ::prost::alloc::vec::Vec<u8>,
    #[prost(message, optional, tag="5")]
    pub signed_block_header: ::core::option::Option<SignedBlockHeader>,
    #[prost(bytes="vec", repeated, tag="6")]
    pub kzg_commitment_inclusion_proof: ::prost::alloc::vec::Vec<::prost::alloc::vec::Vec<u8>>,
    #[prost(message, optional, tag="7")]
    pub extra: ::core::option::Option<Extra>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct SignedBlockHeader {
    #[prost(message, optional, tag="1")]
    pub message: ::core::option::Option<Message>,
    #[prost(bytes="vec", tag="2")]
    pub signature: ::prost::alloc::vec::Vec<u8>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Message {
    #[prost(uint64, tag="1")]
    pub slot: u64,
    #[prost(uint64, tag="2")]
    pub proposer_index: u64,
    #[prost(bytes="vec", tag="3")]
    pub parent_root: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="4")]
    pub state_root: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="5")]
    pub body_root: ::prost::alloc::vec::Vec<u8>,
}
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Extra {
    #[prost(uint64, tag="1")]
    pub block_number: u64,
    #[prost(message, optional, tag="2")]
    pub timestamp: ::core::option::Option<::prost_types::Timestamp>,
}
// @@protoc_insertion_point(module)
