// @generated
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Slot {
    #[prost(uint64, tag="1")]
    pub slot: u64,
    #[prost(bytes="vec", tag="2")]
    pub root: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag="3")]
    pub proposer_index: u64,
    #[prost(bytes="vec", tag="4")]
    pub parent_root: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="5")]
    pub state_root: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="6")]
    pub body_root: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="7")]
    pub signature: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag="8")]
    pub block_number: u64,
    #[prost(message, optional, tag="9")]
    pub timestamp: ::core::option::Option<::prost_types::Timestamp>,
    #[prost(message, repeated, tag="10")]
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
    #[prost(bytes="vec", repeated, tag="6")]
    pub kzg_commitment_inclusion_proof: ::prost::alloc::vec::Vec<::prost::alloc::vec::Vec<u8>>,
}
// @@protoc_insertion_point(module)
