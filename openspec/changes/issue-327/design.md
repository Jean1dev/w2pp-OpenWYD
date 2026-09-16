## Approach

Add `ArchCrystalStage` to the entity and persistence contract (protobuf field 48), with migration `0022_arch_crystal_stage` defaulting existing characters to zero and constraining stages to 0..4. Handle crystal items in the item dispatcher using the single-owner loop. Stage rewards and item consumption in a snapshot, save off-loop with `World.Go`, and publish them only after successful persistence. Use the same persistence-first pattern for Lindy. Block repeated item/combine requests with `UserWaitDB` during the save.

Reuse the existing persisted flat HP/MP values. Reconstruct crystal AC from the stage only while the tier is Arch, avoiding a separate AC column and ensuring the Celestial reset does not regain historical crystal bonuses on login.

Centralize Arch lock checks in EXP application and level-up processing. Lindy remains the only unlock path; it accepts characters already above a boundary, updates the kingdom cape on the first unlock, and persists the result.

Ideal Stone eligibility remains the PR #314 contract. `buildCelestialSnapshot` chooses 3500/3501/3502 from Arch level, but uses 3500 when the crystal stage is below four. The transition remains transactional from the game loop's perspective: save the staged snapshot off-loop, then replace the entity only on success.

## Compatibility

The client wire format is unchanged. New persistence fields use protobuf fields and a nullable-safe migration default. Existing rows are treated as having no completed crystal stages.

## Failure Handling

Rejected crystal, lock, Lindy, and Ideal Stone actions resynchronize the source slot and do not consume items. Save failures leave the live entity unchanged.

## Legacy evidence and deliberate differences

`Source/Code/TMSrv/_MSG_UseItem.cpp:3365` defines crystal sequence, bonuses and 100M EXP deduction. Its class/level checks are commented out; the approved behavior restores Arch and stored level >=355. EXP subtraction clamps at zero without reducing level rather than reproducing unsigned underflow. The item catalog confirms 3500 Prateada, 3501 Dourada, and 3502 Mistica.

`CMob.cpp:1110` and `GetFunc.cpp:1032` define the 354/369 internal gates. The common Go level-up loop checks each iteration, and combat/direct/castle EXP paths check before adding EXP. Dust is preserved with readable Lindy feedback, as approved, instead of being consumed while locked.

`_MSG_CombineItemLindy.cpp:93` changes the cape only on the first unlock; the second consumes one Fame. Approved late recovery uses >= boundaries and completes one missing stage per recipe, without deleveling or automatically granting missing flags.

The local Ideal Stone C++ chooses only by source level. The approved incomplete-crystal penalty overrides that body item to 3500; all other PR #314 prerequisites, band metadata, resets and persistence ordering remain in effect. Readable chat feedback supplements the existing placeholder notices.

## Migration and validation

Apply migrations before running the updated tmserver/dbserver pair. No completion is inferred from old levels or capes. Roll back binaries before dropping the additive stage column; dropping it discards recorded quest history.

Use socket-level regression tests for item/combine packets, stage-by-stage relog, observer cape updates, late unlocks, failed saves and all crystal/Ideal Stone outcomes. Use DBServer and dbclient mapping tests plus an isolated Docker PostgreSQL database for persistence and constraint/transaction validation. Run build, race tests, vet and lint. Actual client rendering is a separate manual check; packet tests verify the server output without claiming to run WYD.exe.
