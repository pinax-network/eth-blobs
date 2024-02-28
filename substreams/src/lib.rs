mod pb;

use pb::pinax::ethereum::blobs::v1::{Blob, Blobs};
use pb::sf::beacon::r#type::v1::{block::Body::*, Block as BeaconBlock};
use substreams::Hex;
use substreams_entity_change::pb::entity::EntityChanges;
use substreams_entity_change::tables::Tables;
use substreams_sink_kv::pb::sf::substreams::sink::kv::v1::KvOperations;

#[substreams::handlers::map]
fn map_blobs(blk: BeaconBlock) -> Result<Blobs, substreams::errors::Error> {
    let blobs = match blk.body.unwrap() {
        Deneb(body) => body
            .embedded_blobs
            .into_iter()
            // .inspect(|b| substreams::log::info!("b.kzg_commitment_inclusion_proof: {:?}", b.kzg_commitment_inclusion_proof))
            .map(|b| Blob {
                index: b.index as u32,
                blob: b.blob,
                kzg_commitment: b.kzg_commitment,
                kzg_proof: b.kzg_proof,
                kzg_commitment_inclusion_proof: b.kzg_commitment_inclusion_proof,

                slot: blk.slot,
                proposer_index: blk.proposer_index,
                parent_root: blk.parent_root.clone(),
                state_root: blk.state_root.clone(),
                body_root: blk.body_root.clone(),
                signature: blk.signature.clone(),

                block_number: body.execution_payload.as_ref().unwrap().block_number,
                timestamp: blk.timestamp.clone(),
                root: blk.root.clone(),
            })
            .collect(),
        _ => vec![],
    };
    Ok(Blobs { blobs })
}

#[substreams::handlers::map]
fn kv_out(blobs: Blobs) -> Result<KvOperations, substreams::errors::Error> {
    let mut kv_ops: KvOperations = Default::default();

    if blobs.blobs.is_empty() {
        return Ok(kv_ops);
    }

    let slot = blobs.blobs.first().as_ref().unwrap().slot;
    let key = format!("slot:{}", slot);
    let value = substreams::proto::encode(&blobs).expect("unable to encode blobs");
    kv_ops.push_new(key, value, 1);

    let block_root_key = format!("block_root:0x{}", Hex::encode(blobs.blobs.first().as_ref().unwrap().root.clone()));
    kv_ops.push_new(block_root_key, slot.to_be_bytes(), 1);

    kv_ops.push_new("head", slot.to_be_bytes(), 1);

    Ok(kv_ops)
}


#[substreams::handlers::map]
fn graph_out(blobs: Blobs) -> Result<EntityChanges, substreams::errors::Error> {
    let mut tables = Tables::new();

    blobs.blobs.iter().for_each(|blob| {
        tables
            .create_row("Blob", format!("{}:{}", blob.slot, blob.index))
            .set("slot", blob.slot)
            .set("index", blob.index)
            .set("data", blob.blob.clone());
    });

    Ok(tables.to_entity_changes())
}
