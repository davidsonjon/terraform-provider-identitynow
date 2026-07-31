# Manages the complete membership list of a governance group. Exactly one
# identitynow_governance_group_members_v1 resource should be created per
# governance group - it reconciles actual membership to match member_ids
# exactly on every apply.
resource "identitynow_governance_group_members_v1" "example" {
  governance_group_id = "2c91808a7813090a017814121919ecca"

  member_ids = [
    "2c7180a46faadee4016fb4e018c20642",
    "2c7180a46faadee4016fb4e018c20643",
  ]
}
