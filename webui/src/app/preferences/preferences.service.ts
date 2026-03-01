import { Injectable, inject } from "@angular/core";
import { BehaviorSubject } from "rxjs";
import { BrowserStorageService } from "../browser-storage/browser-storage.service";
import { TorrentContentOrderByField } from "../graphql/generated";

export interface OrderByPreference {
  field: TorrentContentOrderByField;
  descending: boolean;
}

const LOCAL_STORAGE_KEY = "lodestone-default-order-by";

@Injectable({ providedIn: "root" })
export class PreferencesService {
  private browserStorage = inject(BrowserStorageService);
  private orderBySubject = new BehaviorSubject<OrderByPreference | undefined>(
    this.loadOrderBy(),
  );

  orderBy$ = this.orderBySubject.asObservable();

  getDefaultOrderBy(): OrderByPreference | undefined {
    return this.orderBySubject.getValue();
  }

  setDefaultOrderBy(pref: OrderByPreference | undefined) {
    if (pref) {
      this.browserStorage.set(LOCAL_STORAGE_KEY, JSON.stringify(pref));
    } else {
      this.browserStorage.remove(LOCAL_STORAGE_KEY);
    }
    this.orderBySubject.next(pref);
  }

  private loadOrderBy(): OrderByPreference | undefined {
    const raw = this.browserStorage.get(LOCAL_STORAGE_KEY);
    if (!raw) return undefined;
    try {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed.field === "string" && typeof parsed.descending === "boolean") {
        return parsed as OrderByPreference;
      }
    } catch {
      // ignore
    }
    return undefined;
  }
}
