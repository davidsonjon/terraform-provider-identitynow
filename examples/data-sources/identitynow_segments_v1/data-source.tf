# Lists all segments, capped at 10 results (the `segments` list endpoint has
# no server-side name/attribute filtering, unlike some other plural data
# sources in this provider - see the resource docs' Known Limitations).
data "identitynow_segments_v1" "example" {
  limit = 10
}

output "segment_ids_by_name" {
  value = { for s in data.identitynow_segments_v1.example.segments : s.name => s.id }
}
