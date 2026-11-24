// Shapes returned by the Go API (see docs/API.md).

export type Role = "customer" | "admin" | "owner";
export type OrderStatus = "waiting" | "confirmed" | "shipping" | "delivered" | "rejected";
export type ProductStatus = "draft" | "active" | "archived";

export interface User {
  id: string;
  telegramId: number;
  firstName: string;
  lastName: string;
  username: string;
  phone: string | null;
  role: Role;
  isActive: boolean;
  notificationsEnabled: boolean;
  createdAt: string;
}

export interface Tokens {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

export interface ImageURLs {
  thumb: string;
  card: string;
  full: string;
}

export interface Page<T> {
  items: T[];
  page: number;
  limit: number;
  total: number;
}

export interface Category {
  id: string;
  name: string;
  slug: string;
  position: number;
  isActive: boolean;
  productCount: number;
  imageId: string;
  image: ImageURLs | null;
}

export interface ProductCard {
  id: string;
  slug: string;
  name: string;
  brand: string;
  category: { name: string; slug: string };
  isNew: boolean;
  isFeatured: boolean;
  priceFrom: number;
  oldPriceFrom: number | null;
  inStock: boolean;
  hasVariants: boolean;
  image: ImageURLs | null;
}

export interface PublicVariant {
  id: string;
  label: string;
  price: number;
  oldPrice: number | null;
  inStock: boolean;
  image: ImageURLs | null;
}

export interface ProductDetail {
  id: string;
  slug: string;
  name: string;
  brand: string;
  description: string;
  ingredients: string;
  howToUse: string;
  optionLabel: string;
  category: { name: string; slug: string };
  isNew: boolean;
  priceFrom: number;
  inStock: boolean;
  images: ImageURLs[];
  variants: PublicVariant[];
}

export interface PaymentInfo {
  cardNumber: string;
  cardHolder: string;
  paymentNote: string;
  shopPhone: string;
}

export interface OrderItem {
  productId: string;
  variantId: string;
  productName: string;
  variantLabel: string;
  unitPrice: number;
  quantity: number;
  lineTotal: number;
  image: ImageURLs | null;
}

export interface Order {
  id: string;
  number: number;
  status: OrderStatus;
  customerName: string;
  phone: string;
  addressText: string;
  lat: number;
  lng: number;
  comment: string;
  totalAmount: number;
  items: OrderItem[];
  receiptFileId: string;
  rejectReason: string;
  timeline: { status: OrderStatus; at: string }[];
  createdAt: string;
  updatedAt: string;
}

export interface StaffRef {
  id: string;
  name: string;
}

export interface AdminOrder extends Order {
  customer: { id: string; username: string; telegramId: number };
  assignedAdmin: StaffRef | null;
  receiptDuplicate: { orderId: string; number: number; kind: "exact" | "similar" } | null;
  history: { from: OrderStatus | null; to: OrderStatus; changedBy: string; comment: string; at: string }[];
  notes: { id: string; author: string; body: string; createdAt: string }[];
}

export interface OrderSummary {
  id: string;
  number: number;
  status: OrderStatus;
  customerName: string;
  phone: string;
  totalAmount: number;
  itemCount: number;
  assignedAdmin: StaffRef | null;
  duplicateReceipt: boolean;
  createdAt: string;
}

export interface Stats {
  counts: Record<OrderStatus, number>;
  unassigned: number;
}

export interface AdminVariant {
  id?: string;
  label: string;
  sku: string;
  price: number;
  oldPrice: number | null;
  stock: number;
  imageId: string;
  isActive: boolean;
}

export interface AdminImage {
  id: string;
  urls: ImageURLs;
}

export interface AdminProduct {
  id: string;
  slug: string;
  categoryId: string;
  category: { name: string; slug: string };
  name: string;
  brand: string;
  description: string;
  ingredients: string;
  howToUse: string;
  optionLabel: string;
  status: ProductStatus;
  isNew: boolean;
  isFeatured: boolean;
  images: AdminImage[];
  variants: (AdminVariant & { id: string })[];
  updatedAt: string;
}

export interface AdminProductRow {
  id: string;
  slug: string;
  name: string;
  brand: string;
  status: ProductStatus;
  category: { name: string; slug: string };
  priceFrom: number;
  stockTotal: number;
  variantCount: number;
  isNew: boolean;
  isFeatured: boolean;
  image: ImageURLs | null;
  updatedAt: string;
}

export interface StaffUser extends User {
  lastLoginAt: string | null;
}

export interface AuditRow {
  id: number;
  actor: string;
  action: string;
  entity: string;
  entityId: string;
  meta: Record<string, unknown>;
  createdAt: string;
}

export interface Settings {
  shopName: string;
  shopPhone: string;
  cardNumber: string;
  cardHolder: string;
  paymentNote: string;
}
