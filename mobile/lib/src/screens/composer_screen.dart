import 'dart:convert';
import 'dart:io';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';
import '../api/models.dart';
import '../auth/auth_provider.dart';

class ComposerScreen extends ConsumerStatefulWidget {
  final Post? editPost;
  const ComposerScreen({super.key, this.editPost});
  @override
  ConsumerState<ComposerScreen> createState() => _ComposerScreenState();
}

class _ComposerScreenState extends ConsumerState<ComposerScreen> {
  final titleCtrl = TextEditingController();
  final descCtrl = TextEditingController();
  final tagsCtrl = TextEditingController();
  final ingredientsCtrl = TextEditingController();
  final stepsCtrl = TextEditingController();
  final servingsCtrl = TextEditingController();
  final prepCtrl = TextEditingController();
  final cookCtrl = TextEditingController();
  final scheduledCtrl = TextEditingController();
  bool locked = false;
  String status = 'published';
  XFile? pickedFile;
  bool saving = false;
  String? error;

  @override
  void initState() {
    super.initState();
    final p = widget.editPost;
    if (p != null) {
      titleCtrl.text = p.title;
      descCtrl.text = p.description;
      tagsCtrl.text = p.tags.join(', ');
      locked = p.locked;
      status = p.status;
      if (p.scheduledAt != null) {
        scheduledCtrl.text = p.scheduledAt!.toIso8601String();
      }
      if (!p.recipe.isEmpty) {
        ingredientsCtrl.text = p.recipe.ingredients.join('\n');
        stepsCtrl.text = p.recipe.steps.join('\n');
        if (p.recipe.servings > 0) servingsCtrl.text = '${p.recipe.servings}';
        if (p.recipe.prepMinutes > 0) prepCtrl.text = '${p.recipe.prepMinutes}';
        if (p.recipe.cookMinutes > 0) cookCtrl.text = '${p.recipe.cookMinutes}';
      }
    }
  }

  Future<void> _pick(ImageSource src) async {
    final picker = ImagePicker();
    final f = await picker.pickImage(source: src, imageQuality: 85, maxWidth: 2048);
    if (f != null) setState(() => pickedFile = f);
  }

  Future<void> _pickVideo() async {
    final picker = ImagePicker();
    final f = await picker.pickVideo(source: ImageSource.gallery);
    if (f != null) setState(() => pickedFile = f);
  }

  Map<String, dynamic>? _buildRecipeJson() {
    final ing = ingredientsCtrl.text.split('\n').map((e) => e.trim()).where((e) => e.isNotEmpty).toList();
    final steps = stepsCtrl.text.split('\n').map((e) => e.trim()).where((e) => e.isNotEmpty).toList();
    final servings = int.tryParse(servingsCtrl.text.trim()) ?? 0;
    final prep = int.tryParse(prepCtrl.text.trim()) ?? 0;
    final cook = int.tryParse(cookCtrl.text.trim()) ?? 0;
    if (ing.isEmpty && steps.isEmpty && servings == 0 && prep == 0 && cook == 0) return null;
    return {'ingredients': ing, 'steps': steps, 'servings': servings, 'prep_minutes': prep, 'cook_minutes': cook};
  }

  Future<void> _submit() async {
    final auth = ref.read(authProvider);
    if (!auth.authenticated) {
      setState(() => error = 'Log in to publish posts.');
      return;
    }
    // Edit mode is admin-only
    if (widget.editPost != null && !auth.isAdmin) {
      setState(() => error = 'Only admins can edit existing posts.');
      return;
    }
    if (widget.editPost == null && pickedFile == null) {
      setState(() => error = 'Pick a photo or video');
      return;
    }
    if (titleCtrl.text.trim().isEmpty) {
      setState(() => error = 'Title is required');
      return;
    }
    setState(() { saving = true; error = null; });
    try {
      final api = ref.read(apiClientProvider);
      if (widget.editPost == null) {
        final recipe = _buildRecipeJson();
        final form = FormData.fromMap({
          'title': titleCtrl.text.trim(),
          'description': descCtrl.text.trim(),
          'locked': locked ? 'true' : 'false',
          'status': status,
          'tags': tagsCtrl.text.trim(),
          if (recipe != null) 'recipe': jsonEncode(recipe),
          if (scheduledCtrl.text.trim().isNotEmpty) 'scheduled_at': scheduledCtrl.text.trim(),
          'file': await MultipartFile.fromFile(pickedFile!.path, filename: pickedFile!.name),
        });
        await api.dio.post('/api/posts', data: form);
      } else {
        final recipe = _buildRecipeJson();
        await api.dio.patch('/api/admin/posts/${widget.editPost!.id}', data: {
          'title': titleCtrl.text.trim(),
          'description': descCtrl.text.trim(),
          'locked': locked,
          'status': status,
          'tags': tagsCtrl.text.split(',').map((e) => e.trim()).where((e) => e.isNotEmpty).toList(),
          if (recipe != null) 'recipe': recipe,
          if (scheduledCtrl.text.trim().isNotEmpty) 'scheduled_at': scheduledCtrl.text.trim(),
        });
        if (pickedFile != null) {
          final form = FormData.fromMap({
            'file': await MultipartFile.fromFile(pickedFile!.path, filename: pickedFile!.name),
          });
          await api.dio.put('/api/admin/posts/${widget.editPost!.id}/media', data: form);
        }
      }
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Saved!')));
      context.pop();
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? (e.response!.data['error']?.toString() ?? e.response!.data.toString()) : e.message;
      setState(() => error = msg ?? 'Failed to save');
    } catch (e) {
      setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final auth = ref.watch(authProvider);
    final scheme = Theme.of(context).colorScheme;

    if (!auth.authenticated) {
      return Scaffold(appBar: AppBar(title: const Text('Composer')), body: Center(child: Column(mainAxisSize: MainAxisSize.min, children: [const Icon(Icons.lock_outline, size: 48), const SizedBox(height: 12), const Text('Log in to publish'), const SizedBox(height: 8), FilledButton(onPressed: () => context.go('/login'), child: const Text('Log in'))])));
    }
    // Edit gate: admin only for editing
    if (widget.editPost != null && !auth.isAdmin) {
      return Scaffold(
        appBar: AppBar(title: const Text('Edit post')),
        body: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(mainAxisAlignment: MainAxisAlignment.center, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            Icon(Icons.admin_panel_settings_outlined, size: 56, color: scheme.error),
            const SizedBox(height: 16),
            Text('Admin only', style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w800), textAlign: TextAlign.center),
            const SizedBox(height: 8),
            Text(
              'Editing posts is admin-only. Your account (${auth.me?.email ?? ''}) has role "${auth.me?.role ?? 'normal'}".',
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
            ),
            const SizedBox(height: 16),
            OutlinedButton(onPressed: () => context.pop(), child: const Text('Go back')),
          ]),
        ),
      );
    }

    return Scaffold(
      appBar: AppBar(title: Text(widget.editPost == null ? 'New post' : 'Edit post')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          if (error != null)
            Container(padding: const EdgeInsets.all(12), decoration: BoxDecoration(color: scheme.errorContainer, borderRadius: BorderRadius.circular(12)), child: Text(error!, style: TextStyle(color: scheme.onErrorContainer))),
          if (error != null) const SizedBox(height: 12),
          Container(
            height: 180,
            decoration: BoxDecoration(color: scheme.surfaceContainerHighest, borderRadius: BorderRadius.circular(16), border: Border.all(color: scheme.outlineVariant)),
            child: pickedFile == null
                ? Column(mainAxisAlignment: MainAxisAlignment.center, children: [
                    Icon(Icons.add_a_photo_outlined, size: 36, color: scheme.onSurfaceVariant),
                    const SizedBox(height: 8),
                    Text(widget.editPost == null ? 'No media selected' : 'Keep existing media or pick new'),
                  ])
                : ClipRRect(
                    borderRadius: BorderRadius.circular(16),
                    child: pickedFile!.path.toLowerCase().endsWith('.mp4') || pickedFile!.path.toLowerCase().endsWith('.mov') || pickedFile!.path.toLowerCase().endsWith('.webm')
                        ? Center(child: Column(mainAxisSize: MainAxisSize.min, children: [const Icon(Icons.videocam, size: 36), const SizedBox(height: 6), Text(pickedFile!.name, overflow: TextOverflow.ellipsis)]))
                        : Image.file(File(pickedFile!.path), fit: BoxFit.cover, width: double.infinity, height: double.infinity),
                  ),
          ),
          const SizedBox(height: 8),
          Wrap(spacing: 8, children: [
            FilledButton.icon(onPressed: () => _pick(ImageSource.gallery), icon: const Icon(Icons.photo_library_outlined, size: 18), label: const Text('Gallery')),
            OutlinedButton.icon(onPressed: () => _pick(ImageSource.camera), icon: const Icon(Icons.photo_camera_outlined, size: 18), label: const Text('Camera')),
            OutlinedButton.icon(onPressed: _pickVideo, icon: const Icon(Icons.video_library_outlined, size: 18), label: const Text('Video')),
            if (pickedFile != null) TextButton(onPressed: () => setState(() => pickedFile = null), child: const Text('Clear')),
          ]),
          const SizedBox(height: 16),
          TextField(controller: titleCtrl, decoration: const InputDecoration(labelText: 'Title *', hintText: 'e.g. Spicy Ramen Bowl')),
          const SizedBox(height: 12),
          TextField(controller: descCtrl, decoration: const InputDecoration(labelText: 'Description', hintText: 'A short caption…'), maxLines: 3),
          const SizedBox(height: 12),
          TextField(controller: tagsCtrl, decoration: const InputDecoration(labelText: 'Tags (comma-separated)', hintText: 'ramen, spicy, dinner')),
          const SizedBox(height: 12),
          SwitchListTile(value: locked, onChanged: (v) => setState(() => locked = v), title: const Text('Locked (paywalled)'), subtitle: const Text('Require subscription or per-item purchase')),
          DropdownButtonFormField<String>(
            initialValue: status,
            decoration: const InputDecoration(labelText: 'Status'),
            items: const [DropdownMenuItem(value: 'published', child: Text('Published')), DropdownMenuItem(value: 'draft', child: Text('Draft'))],
            onChanged: (v) => setState(() => status = v ?? 'published'),
          ),
          const SizedBox(height: 12),
          TextField(controller: scheduledCtrl, decoration: const InputDecoration(labelText: 'Scheduled at (optional)', hintText: '2026-12-31T10:00:00Z')),
          const SizedBox(height: 16),
          ExpansionTile(title: const Text('Recipe (optional)', style: TextStyle(fontWeight: FontWeight.w700)), children: [
            Padding(
              padding: const EdgeInsets.all(12),
              child: Column(children: [
                TextField(controller: ingredientsCtrl, decoration: const InputDecoration(labelText: 'Ingredients (one per line)'), maxLines: 4),
                const SizedBox(height: 8),
                TextField(controller: stepsCtrl, decoration: const InputDecoration(labelText: 'Steps (one per line)'), maxLines: 4),
                const SizedBox(height: 8),
                Row(children: [
                  Expanded(child: TextField(controller: servingsCtrl, decoration: const InputDecoration(labelText: 'Servings'), keyboardType: TextInputType.number)),
                  const SizedBox(width: 8),
                  Expanded(child: TextField(controller: prepCtrl, decoration: const InputDecoration(labelText: 'Prep min'), keyboardType: TextInputType.number)),
                  const SizedBox(width: 8),
                  Expanded(child: TextField(controller: cookCtrl, decoration: const InputDecoration(labelText: 'Cook min'), keyboardType: TextInputType.number)),
                ]),
              ]),
            ),
          ]),
          const SizedBox(height: 20),
          FilledButton(
            onPressed: saving ? null : _submit,
            child: saving ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white)) : Text(widget.editPost == null ? 'Publish post' : 'Save changes'),
          ),
          const SizedBox(height: 12),
          Text(widget.editPost == null ? 'Publishes via POST /api/posts (multipart). Any logged-in user can publish.' : 'Editing via PATCH /api/admin/posts/{id} + media replace (admin only).',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant), textAlign: TextAlign.center),
        ]),
      ),
    );
  }
}
