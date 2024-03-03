// @generated
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Slot {
    #[prost(uint64, tag="1")]
    pub slot: u64,
    #[prost(enumeration="Spec", tag="2")]
    pub spec: i32,
    #[prost(bytes="vec", tag="3")]
    pub root: ::prost::alloc::vec::Vec<u8>,
    #[prost(uint64, tag="4")]
    pub proposer_index: u64,
    #[prost(bytes="vec", tag="5")]
    pub parent_root: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="6")]
    pub state_root: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="7")]
    pub body_root: ::prost::alloc::vec::Vec<u8>,
    #[prost(bytes="vec", tag="8")]
    pub signature: ::prost::alloc::vec::Vec<u8>,
    #[prost(message, optional, tag="10")]
    pub timestamp: ::core::option::Option<::prost_types::Timestamp>,
    #[prost(message, repeated, tag="20")]
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
#[allow(clippy::derive_partial_eq_without_eq)]
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct SlotInfo {
    #[prost(uint64, tag="1")]
    pub slot: u64,
    #[prost(message, optional, tag="2")]
    pub timestamp: ::core::option::Option<::prost_types::Timestamp>,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
#[repr(i32)]
pub enum Spec {
    Unspecified = 0,
    Phase0 = 1,
    Altair = 2,
    Bellatrix = 3,
    Capella = 4,
    Deneb = 5,
}
impl Spec {
    /// String value of the enum field names used in the ProtoBuf definition.
    ///
    /// The values are not transformed in any way and thus are considered stable
    /// (if the ProtoBuf definition does not change) and safe for programmatic use.
    pub fn as_str_name(&self) -> &'static str {
        match self {
            Spec::Unspecified => "UNSPECIFIED",
            Spec::Phase0 => "PHASE0",
            Spec::Altair => "ALTAIR",
            Spec::Bellatrix => "BELLATRIX",
            Spec::Capella => "CAPELLA",
            Spec::Deneb => "DENEB",
        }
    }
    /// Creates an enum from field names used in the ProtoBuf definition.
    pub fn from_str_name(value: &str) -> ::core::option::Option<Self> {
        match value {
            "UNSPECIFIED" => Some(Self::Unspecified),
            "PHASE0" => Some(Self::Phase0),
            "ALTAIR" => Some(Self::Altair),
            "BELLATRIX" => Some(Self::Bellatrix),
            "CAPELLA" => Some(Self::Capella),
            "DENEB" => Some(Self::Deneb),
            _ => None,
        }
    }
}
// @@protoc_insertion_point(module)
