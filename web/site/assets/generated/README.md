# Original decorative concept photographs

Generated on 2026-10-09 with the built-in `image_gen` tool, default built-in generation mode, opaque background. Each final image is 1536 × 1024 pixels. These images are decorative concept photographs, **not a product screenshot** and not evidence of HarnessScope behavior.

Every final image was visually inspected for accidental readable text, logos, people/faces, apparent private data and fake interfaces. All selected images pass that inspection. The first local-machine candidate was rejected for apparent book-spine lettering and regenerated with the same prompt; no crop or image edit was used to conceal the defect. All production references use repository-local files.

## observatory-hero.png

Role: decorative hero atmosphere beside the genuine synthetic dashboard capture; not a product screenshot.

Final prompt:

> photorealistic-natural; wide editorial photograph of an empty configuration-forensics observatory workstation, smoked glass, dark navy instrument panels without readable UI, translucent cable paths, cyan instrument light, one amber evidence marker, warm mineral paper notes with no legible writing, dramatic negative space, refined technical magazine photography; no person, no logo, no watermark, no readable text, no fake software interface.

Built-in output: `01a11f80-fe2e-76d2-9b5e-df9d215549b1/exec-3c23c200-2c40-4e8a-af06-44555ccd42b1.png`. Copied unchanged into this directory after checking the destination did not exist.

## evidence-lab.png

Role: decorative material metaphor for inspecting configuration provenance; not a product screenshot.

Final prompt:

> photorealistic-natural; macro editorial photograph of layered translucent sheets, fine connection traces, metal clips and one coral conflict marker on a dark evidence table, cyan edge lighting, tactile materials, shallow depth of field; no person, no logo, no watermark, no readable text, no computer interface.

Built-in output: `01a11f80-fe2e-76d2-9b5e-df9d215549b1/exec-cf145c9d-b692-48fc-9412-6ed9b99c329d.png`. Copied unchanged into this directory after checking the destination did not exist.

## local-machine.png

Role: decorative local/offline workstation atmosphere; not a product screenshot.

Final prompt:

> photorealistic-natural; quiet developer workstation in a dim observatory room, one closed local machine beside a disconnected network cable and physical notebook, navy and warm paper palette, cyan status light, architectural shadows; no person, no brand, no watermark, no readable screen or text.

Built-in selected output: `01a11f80-fe2e-76d2-9b5e-df9d215549b1/exec-b2df13f9-c087-45b5-a382-bf73022ce3d5.png`. Copied unchanged after removing the rejected provisional asset from the project and checking that the destination was absent. Rejected output: `exec-a7c5aedf-d660-47f1-a1a4-54de3cc8f9b4.png` (apparent book-spine lettering); the original remains in the tool's generated-image directory.

## Separate product evidence

`../product/dashboard-overview.png` is copied byte-for-byte from `docs/assets/dashboard-overview.png`. It is a committed Playwright synthetic fixture capture, not generated artwork. The home page visibly distinguishes that capture from the concept photographs and supplies a translated descriptive image alternative.
