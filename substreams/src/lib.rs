mod pb;

use pb::pinax::ethereum::blobs::v1::{Blob, Slot, Spec};
use pb::sf::beacon::r#type::v1::{block::Body::*, Block as BeaconBlock};
use substreams::Hex;
use substreams_entity_change::{pb::entity::EntityChanges, tables::Tables};
use substreams_sink_kv::pb::sf::substreams::sink::kv::v1::KvOperations;

#[substreams::handlers::map]
fn map_blobs(blk: BeaconBlock) -> Result<Slot, substreams::errors::Error> {
    let blobs = match blk.body.unwrap() {
        Deneb(body) => body
            .embedded_blobs
            .into_iter()
            .map(|b| Blob {
                index: b.index as u32,
                blob: b.blob,
                kzg_commitment: b.kzg_commitment,
                kzg_proof: b.kzg_proof,
                kzg_commitment_inclusion_proof: b.kzg_commitment_inclusion_proof,
            })
            .collect(),
        Electra(body) => body
            .embedded_blobs
            .into_iter()
            .map(|b| Blob {
                index: b.index as u32,
                blob: b.blob,
                kzg_commitment: b.kzg_commitment,
                kzg_proof: b.kzg_proof,
                kzg_commitment_inclusion_proof: b.kzg_commitment_inclusion_proof,
            })
            .collect(),
        _ => vec![],
    };

    Ok(Slot {
        slot: blk.slot,
        spec: blk.spec,
        proposer_index: blk.proposer_index,
        parent_root: blk.parent_root.clone(),
        state_root: blk.state_root.clone(),
        body_root: blk.body_root.clone(),
        signature: blk.signature.clone(),
        root: blk.root.clone(),
        timestamp: blk.timestamp.clone(),

        blobs,
    })
}

#[substreams::handlers::map]
fn kv_out(slot: Slot) -> Result<KvOperations, substreams::errors::Error> {
    let mut kv_ops: KvOperations = Default::default();

    let key = format!("slot:{}", slot.slot);
    let value = substreams::proto::encode(&slot).expect("unable to encode slot");
    kv_ops.push_new(key, value, 1);

    let block_root_key = format!("block_root:0x{}", Hex::encode(slot.root.clone()));
    kv_ops.push_new(block_root_key, slot.slot.to_be_bytes(), 1);

    kv_ops.push_new("head", slot.slot.to_be_bytes(), 1);

    Ok(kv_ops)
}

#[substreams::handlers::map]
fn graph_out(slot: Slot) -> Result<EntityChanges, substreams::errors::Error> {
    let mut tables = Tables::new();

    let timestamp = slot.timestamp.unwrap_or_default().to_string();
    let spec = Spec::from_i32(slot.spec).unwrap().as_str_name();

    tables
        .create_row("Slot", format!("{}", slot.slot))
        .set("number", slot.slot)
        .set("timestamp", &timestamp)
        .set("spec", spec)
        .set("proposer_index", slot.proposer_index as i32)
        .set("parent_root", &slot.parent_root)
        .set("state_root", &slot.state_root)
        .set("body_root", &slot.body_root)
        .set("signature", &slot.signature);

    slot.blobs.into_iter().for_each(|blob| {
        tables
            .create_row("Blob", format!("{}:{:0>2}", slot.slot, blob.index))
            .set("slot", slot.slot.to_string())
            .set("index", blob.index as i32)
            .set("blob", blob.blob)
            .set("kzg_commitment", blob.kzg_commitment)
            .set("kzg_proof", blob.kzg_proof)
            .set(
                "kzg_commitment_inclusion_proof",
                blob.kzg_commitment_inclusion_proof,
            );
    });

    Ok(tables.to_entity_changes())
}
