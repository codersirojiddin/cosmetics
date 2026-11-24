"use client";

import "leaflet/dist/leaflet.css";
import L from "leaflet";
import { useEffect, useRef, useState } from "react";
import { Crosshair } from "lucide-react";
import { Button } from "./ui";

export interface LatLng {
  lat: number;
  lng: number;
}

const TASHKENT: [number, number] = [41.3111, 69.2797];

/** Tap the map (or drag the pin) to set the delivery point, or use the device location. */
export default function LocationPicker({ value, onChange }: { value: LatLng | null; onChange: (v: LatLng) => void }) {
  const el = useRef<HTMLDivElement>(null);
  const map = useRef<L.Map | null>(null);
  const marker = useRef<L.Marker | null>(null);
  const cb = useRef(onChange);
  cb.current = onChange;
  const [geoError, setGeoError] = useState<string | null>(null);
  const [locating, setLocating] = useState(false);

  useEffect(() => {
    const m = L.map(el.current!, { center: value ? [value.lat, value.lng] : TASHKENT, zoom: value ? 16 : 11 });
    L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", { maxZoom: 19, attribution: "© OpenStreetMap" }).addTo(m);
    const icon = L.divIcon({ className: "", html: '<div class="pin"></div>', iconSize: [18, 18], iconAnchor: [9, 9] });
    const place = (lat: number, lng: number) => {
      if (marker.current) marker.current.setLatLng([lat, lng]);
      else {
        marker.current = L.marker([lat, lng], { icon, draggable: true }).addTo(m);
        marker.current.on("dragend", () => {
          const p = marker.current!.getLatLng();
          cb.current({ lat: p.lat, lng: p.lng });
        });
      }
    };
    m.on("click", (e: L.LeafletMouseEvent) => {
      place(e.latlng.lat, e.latlng.lng);
      cb.current({ lat: e.latlng.lat, lng: e.latlng.lng });
    });
    if (value) place(value.lat, value.lng);
    map.current = m;
    return () => {
      m.remove();
      map.current = null;
      marker.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function locate() {
    setGeoError(null);
    if (!navigator.geolocation) {
      setGeoError("Brauzeringiz joylashuvni aniqlay olmaydi. Xaritadan belgilang.");
      return;
    }
    setLocating(true);
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLocating(false);
        const v = { lat: pos.coords.latitude, lng: pos.coords.longitude };
        map.current?.setView([v.lat, v.lng], 17);
        if (marker.current) marker.current.setLatLng([v.lat, v.lng]);
        else {
          const icon = L.divIcon({ className: "", html: '<div class="pin"></div>', iconSize: [18, 18], iconAnchor: [9, 9] });
          marker.current = L.marker([v.lat, v.lng], { icon, draggable: true }).addTo(map.current!);
          marker.current.on("dragend", () => {
            const p = marker.current!.getLatLng();
            cb.current({ lat: p.lat, lng: p.lng });
          });
        }
        cb.current(v);
      },
      () => {
        setLocating(false);
        setGeoError("Joylashuvni aniqlab bo'lmadi (ruxsat berilmagan bo'lishi mumkin). Xaritadan belgilang.");
      },
      { enableHighAccuracy: true, timeout: 12000 },
    );
  }

  return (
    <div>
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <Button variant="secondary" size="sm" onClick={locate} loading={locating}>
          <Crosshair className="size-4" /> Joylashuvimni aniqlash
        </Button>
        <span className="text-[12px] text-muted">{value ? `${value.lat.toFixed(5)}, ${value.lng.toFixed(5)}` : "Xaritada bosib belgilang"}</span>
      </div>
      <div ref={el} className="z-0 h-64 w-full border border-line bg-paper" role="application" aria-label="Yetkazib berish nuqtasini tanlash xaritasi" />
      {geoError && <p className="mt-2 text-[12px] text-bad">{geoError}</p>}
    </div>
  );
}
