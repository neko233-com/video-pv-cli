using System.IO;
using UnityEngine;
using UnityEngine.UI;
using UnityEngine.Video;

// Attach to a dedicated RawImage. Copy StreamingAssets/pv.mp4 to Assets/StreamingAssets.
[RequireComponent(typeof(RawImage), typeof(VideoPlayer))]
public sealed class PVVideoSurface : MonoBehaviour
{
    [SerializeField] private bool loop;
    [SerializeField] private bool playAudio = true;
    private VideoPlayer player;
    private RawImage surface;
    private RenderTexture texture;

    private void OnEnable()
    {
        surface = GetComponent<RawImage>();
        surface.raycastTarget = false;
        player = GetComponent<VideoPlayer>();
        texture = new RenderTexture({{WIDTH}}, {{HEIGHT}}, 0, RenderTextureFormat.ARGB32);
        texture.Create();
        surface.texture = texture;
        player.playOnAwake = false;
        player.waitForFirstFrame = true;
        player.source = VideoSource.Url;
        player.url = Path.Combine(Application.streamingAssetsPath, "pv.mp4");
        player.renderMode = VideoRenderMode.RenderTexture;
        player.targetTexture = texture;
        player.audioOutputMode = playAudio ? VideoAudioOutputMode.Direct : VideoAudioOutputMode.None;
        player.isLooping = loop;
        player.prepareCompleted += OnPrepared;
        player.errorReceived += OnError;
        player.Prepare();
    }

    private void OnPrepared(VideoPlayer source) { source.Play(); }
    private void OnError(VideoPlayer source, string message) { Debug.LogError(message, this); }

    private void OnDisable()
    {
        if (player != null)
        {
            player.prepareCompleted -= OnPrepared;
            player.errorReceived -= OnError;
            player.Stop();
            player.targetTexture = null;
        }
        if (surface != null) surface.texture = null;
        if (texture != null) { texture.Release(); Destroy(texture); texture = null; }
    }
}
